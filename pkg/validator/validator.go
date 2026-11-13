package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xdearboy/free-domains/pkg/models"
)

var (
	ErrEmptyRecords  = errors.New("at least one DNS record type must be specified")
	ErrCNAMEConflict = errors.New("CNAME record cannot coexist with other record types")
	ErrMissingOwner  = errors.New("owner.username is required")
	ErrInBailiwickNS = errors.New("in-bailiwick nameservers require out-of-bailiwick delegation")
	ErrSingleNS      = errors.New("delegation requires at least two nameservers")

	labelPattern    = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	singleChar      = regexp.MustCompile(`^[a-z0-9]$`)
	hostnamePattern = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}\.?$`)
	usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$`)

	restrictedIPv4Blocks = []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.88.99.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"255.255.255.255/32",
	}
)

type Validator struct {
	Blacklist       models.BlacklistConfig
	Reserved        models.ReservedConfig
	compiledPrivate []*net.IPNet
}

func NewValidator(blacklistPath, reservedPath string) (*Validator, error) {
	v := &Validator{
		Reserved: make(models.ReservedConfig),
	}

	if blacklistPath != "" {
		raw, err := os.ReadFile(blacklistPath)
		if err != nil {
			return nil, fmt.Errorf("read blacklist %s: %w", blacklistPath, err)
		}
		if err := json.Unmarshal(raw, &v.Blacklist); err != nil {
			return nil, fmt.Errorf("parse blacklist %s: %w", blacklistPath, err)
		}
	}

	if reservedPath != "" {
		raw, err := os.ReadFile(reservedPath)
		if err != nil {
			return nil, fmt.Errorf("read reserved %s: %w", reservedPath, err)
		}
		if err := json.Unmarshal(raw, &v.Reserved); err != nil {
			return nil, fmt.Errorf("parse reserved %s: %w", reservedPath, err)
		}
	}

	for _, block := range restrictedIPv4Blocks {
		_, ipNet, err := net.ParseCIDR(block)
		if err != nil {
			return nil, fmt.Errorf("parse cidr %s: %w", block, err)
		}
		v.compiledPrivate = append(v.compiledPrivate, ipNet)
	}

	return v, nil
}

func (v *Validator) ValidateFile(path, gitActor string) (*models.DomainConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg models.DomainConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	cleanPath := filepath.Clean(path)
	parts := strings.Split(cleanPath, string(filepath.Separator))
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid path layout %s, expected domains/<zone>/<shard>/<name>.json", path)
	}

	file := parts[len(parts)-1]
	shard := parts[len(parts)-2]
	zone := parts[len(parts)-3]

	if !strings.HasSuffix(file, ".json") {
		return nil, fmt.Errorf("invalid file extension for %s", file)
	}

	sub := strings.TrimSuffix(file, ".json")
	cfg.Subdomain = sub
	cfg.Zone = zone
	cfg.FilePath = cleanPath

	expectedShard := string(sub[0])
	if shard != expectedShard {
		return nil, fmt.Errorf("sharding mismatch: %s should be in %s/, got %s/", sub, expectedShard, shard)
	}

	if err := v.ValidateConfig(&cfg, gitActor); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return &cfg, nil
}

func (v *Validator) ValidateConfig(cfg *models.DomainConfig, gitActor string) error {
	sub := cfg.Subdomain
	zone := cfg.Zone

	if sub != strings.ToLower(sub) {
		return fmt.Errorf("subdomain %q must be lowercase", sub)
	}

	if len(sub) == 1 {
		if !singleChar.MatchString(sub) {
			return fmt.Errorf("invalid single-char label %q", sub)
		}
	} else if !labelPattern.MatchString(sub) {
		return fmt.Errorf("label %q violates RFC 1035 LDH rules", sub)
	}

	if len(sub) >= 4 && sub[2:4] == "--" {
		return fmt.Errorf("label %q contains reserved punycode marker", sub)
	}

	fqdn := fmt.Sprintf("%s.%s", sub, zone)
	if len(fqdn) > 253 {
		return fmt.Errorf("fqdn %q exceeds maximum length 253", fqdn)
	}

	if err := v.checkRestrictedNames(sub, zone); err != nil {
		return err
	}

	username := strings.TrimSpace(cfg.Owner.Username)
	if username == "" {
		return ErrMissingOwner
	}
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf("invalid github username %q", username)
	}

	if gitActor != "" && !strings.EqualFold(username, gitActor) {
		return fmt.Errorf("identity mismatch: actor %q cannot modify records owned by %q", gitActor, username)
	}

	if cfg.TTL == 0 {
		cfg.TTL = 300
	} else if cfg.TTL < 60 || cfg.TTL > 86400 {
		return fmt.Errorf("ttl %d out of bounds [60, 86400]", cfg.TTL)
	}

	return v.validateRecords(cfg, fqdn)
}

func (v *Validator) validateRecords(cfg *models.DomainConfig, fqdn string) error {
	r := cfg.Records
	hasA := len(r.A) > 0
	hasAAAA := len(r.AAAA) > 0
	hasCNAME := strings.TrimSpace(r.CNAME) != ""
	hasTXT := len(r.TXT) > 0
	hasNS := len(r.NS) > 0

	if !hasA && !hasAAAA && !hasCNAME && !hasTXT && !hasNS {
		return ErrEmptyRecords
	}

	if hasCNAME {
		if hasA || hasAAAA || hasTXT || hasNS {
			return ErrCNAMEConflict
		}
		if err := v.validateHost(r.CNAME); err != nil {
			return fmt.Errorf("cname target: %w", err)
		}
		if strings.TrimSuffix(strings.ToLower(r.CNAME), ".") == strings.ToLower(fqdn) {
			return fmt.Errorf("self-referencing cname target %q", r.CNAME)
		}
	}

	if hasNS {
		if hasA || hasAAAA || hasTXT {
			return fmt.Errorf("delegation cannot coexist with address or txt records")
		}
		if len(r.NS) < 2 {
			return ErrSingleNS
		}
		subSuffix := "." + strings.ToLower(fqdn)
		for _, target := range r.NS {
			if err := v.validateHost(target); err != nil {
				return fmt.Errorf("ns target: %w", err)
			}
			norm := strings.TrimSuffix(strings.ToLower(target), ".")
			if strings.HasSuffix(norm, subSuffix) || norm == strings.ToLower(fqdn) {
				return ErrInBailiwickNS
			}
		}
	}

	for _, addr := range r.A {
		ip := net.ParseIP(addr)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("malformed ipv4 address %q", addr)
		}
		v4 := ip.To4()
		for _, block := range v.compiledPrivate {
			if block.Contains(v4) {
				return fmt.Errorf("prohibited address %q in subnet %s", addr, block)
			}
		}
	}

	for _, addr := range r.AAAA {
		ip := net.ParseIP(addr)
		if ip == nil || ip.To4() != nil {
			return fmt.Errorf("malformed ipv6 address %q", addr)
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
			return fmt.Errorf("prohibited loopback or link-local ipv6 address %q", addr)
		}
	}

	for _, txt := range r.TXT {
		if len(txt) > 255 {
			return fmt.Errorf("txt record exceeds 255 octets: length %d", len(txt))
		}
	}

	if len(r.AcmeChallenge) > 255 {
		return fmt.Errorf("acme challenge exceeds 255 octets")
	}

	return nil
}

func (v *Validator) checkRestrictedNames(sub, zone string) error {
	norm := strings.ToLower(sub)

	for _, item := range v.Blacklist.Exact {
		if norm == strings.ToLower(item) {
			return fmt.Errorf("subdomain %q is blacklisted", sub)
		}
	}

	for _, p := range v.Blacklist.Patterns {
		matched, err := regexp.MatchString("^"+p+"$", norm)
		if err == nil && matched {
			return fmt.Errorf("subdomain %q matches prohibited pattern %q", sub, p)
		}
	}

	if reserved, exists := v.Reserved[zone]; exists {
		for _, item := range reserved {
			if norm == strings.ToLower(item) {
				return fmt.Errorf("subdomain %q is reserved for zone %q", sub, zone)
			}
		}
	}

	return nil
}

func (v *Validator) validateHost(host string) error {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return errors.New("hostname is empty")
	}
	if len(trimmed) > 253 {
		return errors.New("hostname exceeds 253 characters")
	}
	if !hostnamePattern.MatchString(trimmed) {
		return fmt.Errorf("invalid hostname format %q", host)
	}
	return nil
}
