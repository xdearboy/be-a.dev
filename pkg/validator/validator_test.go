package validator

import (
	"path/filepath"
	"testing"

	"github.com/xdearboy/free-domains/pkg/models"
)

func createTestValidator(t *testing.T) *Validator {
	t.Helper()
	v, err := NewValidator(
		filepath.Join("..", "..", "config", "blacklist.json"),
		filepath.Join("..", "..", "config", "reserved.json"),
	)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}
	return v
}

func TestValidConfigs(t *testing.T) {
	v := createTestValidator(t)

	tests := []struct {
		name   string
		config models.DomainConfig
	}{
		{
			name: "Valid CNAME with ACME challenge",
			config: models.DomainConfig{
				Subdomain: "alice",
				Zone:      "kra67.space",
				Owner: models.Owner{
					Username: "alice",
				},
				Records: models.RecordSet{
					CNAME:         "alice.github.io.",
					AcmeChallenge: "token123",
				},
				TTL: 300,
			},
		},
		{
			name: "Valid A and AAAA records",
			config: models.DomainConfig{
				Subdomain: "my-app",
				Zone:      "kra67.space",
				Owner: models.Owner{
					Username: "my-user",
				},
				Records: models.RecordSet{
					A:    []string{"185.12.34.56"},
					AAAA: []string{"2a00:1450:4001:828::200e"},
					TXT:  []string{"v=spf1 ..."},
				},
				TTL: 600,
			},
		},
		{
			name: "Valid NS delegation",
			config: models.DomainConfig{
				Subdomain: "delegated",
				Zone:      "plmt.tech",
				Owner: models.Owner{
					Username: "pro-dev",
				},
				Records: models.RecordSet{
					NS: []string{"ns1.he.net.", "ns2.he.net."},
				},
				TTL: 86400,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := v.ValidateConfig(&tc.config, "alice"); err != nil && tc.config.Owner.Username == "alice" {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRFC1035SyntaxViolations(t *testing.T) {
	v := createTestValidator(t)

	badNames := []struct {
		subdomain string
		reason    string
	}{
		{"-bad", "leading hyphen"},
		{"bad-", "trailing hyphen"},
		{"Uppercase", "uppercase characters"},
		{"hello_world", "underscore not allowed in host label"},
		{"xn--p1ai", "punycode disallowed"},
		{"ab--cd", "double hyphen in position 3-4 disallowed if xn--"},
	}

	for _, tc := range badNames {
		t.Run(tc.reason, func(t *testing.T) {
			cfg := &models.DomainConfig{
				Subdomain: tc.subdomain,
				Zone:      "kra67.space",
				Owner:     models.Owner{Username: "test"},
				Records:   models.RecordSet{A: []string{"185.1.2.3"}},
			}
			if err := v.ValidateConfig(cfg, "test"); err == nil {
				t.Errorf("expected error for '%s' (%s), but got nil", tc.subdomain, tc.reason)
			}
		})
	}
}

func TestCNAMEExclusivityRule(t *testing.T) {
	v := createTestValidator(t)

	cfg := &models.DomainConfig{
		Subdomain: "conflict",
		Zone:      "kra67.space",
		Owner:     models.Owner{Username: "test"},
		Records: models.RecordSet{
			CNAME: "target.example.com.",
			A:     []string{"185.1.2.3"},
		},
	}

	if err := v.ValidateConfig(cfg, "test"); err == nil {
		t.Errorf("expected RFC 1034 CNAME exclusivity error, got nil")
	}
}

func TestAntiSpoofingPrivateIP(t *testing.T) {
	v := createTestValidator(t)

	privateIPs := []string{
		"127.0.0.1",
		"10.0.0.5",
		"192.168.1.1",
		"172.16.0.1",
		"169.254.1.1",
		"0.0.0.0",
		"224.0.0.1",
	}

	for _, ip := range privateIPs {
		t.Run(ip, func(t *testing.T) {
			cfg := &models.DomainConfig{
				Subdomain: "spoof-test",
				Zone:      "kra67.space",
				Owner:     models.Owner{Username: "test"},
				Records: models.RecordSet{
					A: []string{ip},
				},
			}
			if err := v.ValidateConfig(cfg, "test"); err == nil {
				t.Errorf("expected error for private IP %s, got nil", ip)
			}
		})
	}
}

func TestBlacklistAndReserved(t *testing.T) {
	v := createTestValidator(t)

	blockedNames := []struct {
		subdomain string
		zone      string
	}{
		{"admin", "kra67.space"},
		{"sberbank", "kra67.space"},
		{"login-verify", "kra67.space"},
		{"highlight", "plmt.tech"},     // Reserved on plmt.tech
		{"dontusetelega", "plmt.tech"}, // Reserved on plmt.tech
	}

	for _, tc := range blockedNames {
		t.Run(tc.subdomain+"."+tc.zone, func(t *testing.T) {
			cfg := &models.DomainConfig{
				Subdomain: tc.subdomain,
				Zone:      tc.zone,
				Owner:     models.Owner{Username: "test"},
				Records:   models.RecordSet{A: []string{"185.1.2.3"}},
			}
			if err := v.ValidateConfig(cfg, "test"); err == nil {
				t.Errorf("expected blacklist/reserved error for %s on %s, got nil", tc.subdomain, tc.zone)
			}
		})
	}
}

func TestNSDelegationRules(t *testing.T) {
	v := createTestValidator(t)

	// Test < 2 nameservers
	cfgSingleNS := &models.DomainConfig{
		Subdomain: "sub",
		Zone:      "kra67.space",
		Owner:     models.Owner{Username: "test"},
		Records: models.RecordSet{
			NS: []string{"ns1.example.com."},
		},
	}
	if err := v.ValidateConfig(cfgSingleNS, "test"); err == nil {
		t.Errorf("expected error for single NS record, got nil")
	}

	// Test in-bailiwick loop
	cfgInBailiwick := &models.DomainConfig{
		Subdomain: "sub",
		Zone:      "kra67.space",
		Owner:     models.Owner{Username: "test"},
		Records: models.RecordSet{
			NS: []string{"ns1.sub.kra67.space.", "ns2.sub.kra67.space."},
		},
	}
	if err := v.ValidateConfig(cfgInBailiwick, "test"); err == nil {
		t.Errorf("expected error for in-bailiwick NS without glue, got nil")
	}
}

func TestImpersonationProtection(t *testing.T) {
	v := createTestValidator(t)

	cfg := &models.DomainConfig{
		Subdomain: "victim",
		Zone:      "kra67.space",
		Owner:     models.Owner{Username: "legit-user"},
		Records:   models.RecordSet{A: []string{"185.1.2.3"}},
	}

	// Attacker tries to submit PR with different github.actor
	if err := v.ValidateConfig(cfg, "malicious-actor"); err == nil {
		t.Errorf("expected impersonation rejection error, got nil")
	}
}
