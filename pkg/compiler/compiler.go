package compiler

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"

	"github.com/xdearboy/free-domains/pkg/models"
	"github.com/xdearboy/free-domains/pkg/validator"
)

type NameserverDefinition struct {
	Host string
	IPv4 string
}

var DefaultNameservers = []NameserverDefinition{
	{Host: "ns1", IPv4: "130.78.191.155"},
	{Host: "ns2", IPv4: "191.44.118.80"},
	{Host: "ns3", IPv4: "222.167.248.169"},
}

type Compiler struct {
	Validator   *validator.Validator
	Nameservers []NameserverDefinition
}

func NewCompiler(v *validator.Validator) *Compiler {
	return &Compiler{
		Validator:   v,
		Nameservers: DefaultNameservers,
	}
}

func (c *Compiler) CompileDirectory(domainsDir, outDir string) (map[string]int, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("create output directory %s: %w", outDir, err)
	}

	zoneConfigs := make(map[string][]*models.DomainConfig)

	err := filepath.Walk(domainsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		cfg, err := c.Validator.ValidateFile(path, "")
		if err != nil {
			return fmt.Errorf("validate %s: %w", path, err)
		}

		zoneConfigs[cfg.Zone] = append(zoneConfigs[cfg.Zone], cfg)
		return nil
	})

	if err != nil {
		return nil, err
	}

	results := make(map[string]int)
	for zone, configs := range zoneConfigs {
		sort.Slice(configs, func(i, j int) bool {
			return configs[i].Subdomain < configs[j].Subdomain
		})

		content, err := c.GenerateZone(zone, configs)
		if err != nil {
			return nil, fmt.Errorf("generate zone %s: %w", zone, err)
		}

		if err := c.VerifyZone(zone, content); err != nil {
			return nil, fmt.Errorf("verify zone %s: %w", zone, err)
		}

		zoneFilePath := filepath.Join(outDir, fmt.Sprintf("db.%s", zone))
		if err := os.WriteFile(zoneFilePath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("write zone file %s: %w", zoneFilePath, err)
		}

		results[zone] = len(configs)
	}

	return results, nil
}

func (c *Compiler) GenerateZone(zone string, configs []*models.DomainConfig) (string, error) {
	origin := dns.Fqdn(zone)
	now := time.Now().UTC()
	serial := uint32(now.Year()*1000000 + int(now.Month())*10000 + now.Day()*100 + 1)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("$ORIGIN %s\n", origin))
	sb.WriteString("$TTL 300\n\n")

	primaryNS := dns.Fqdn(fmt.Sprintf("%s.%s", c.Nameservers[0].Host, zone))
	adminEmail := dns.Fqdn(fmt.Sprintf("admin.%s", zone))

	soa := &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   origin,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		Ns:      primaryNS,
		Mbox:    adminEmail,
		Serial:  serial,
		Refresh: 3600,
		Retry:   600,
		Expire:  604800,
		Minttl:  300,
	}
	sb.WriteString(soa.String() + "\n\n")

	for _, ns := range c.Nameservers {
		nsFqdn := dns.Fqdn(fmt.Sprintf("%s.%s", ns.Host, zone))
		nsRR := &dns.NS{
			Hdr: dns.RR_Header{
				Name:   origin,
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    86400,
			},
			Ns: nsFqdn,
		}
		sb.WriteString(nsRR.String() + "\n")
	}
	sb.WriteString("\n")

	for _, ns := range c.Nameservers {
		glueFqdn := dns.Fqdn(fmt.Sprintf("%s.%s", ns.Host, zone))
		aRR := &dns.A{
			Hdr: dns.RR_Header{
				Name:   glueFqdn,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    86400,
			},
			A: net.ParseIP(ns.IPv4),
		}
		sb.WriteString(aRR.String() + "\n")
	}
	sb.WriteString("\n")

	for _, ns := range c.Nameservers {
		apexA := &dns.A{
			Hdr: dns.RR_Header{
				Name:   origin,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: net.ParseIP(ns.IPv4),
		}
		sb.WriteString(apexA.String() + "\n")
	}
	wwwRR := &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(fmt.Sprintf("www.%s", zone)),
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		Target: origin,
	}
	sb.WriteString(wwwRR.String() + "\n\n")

	for _, cfg := range configs {
		subdomainFqdn := dns.Fqdn(fmt.Sprintf("%s.%s", cfg.Subdomain, zone))
		ttl := uint32(cfg.TTL)
		if ttl == 0 {
			ttl = 300
		}

		records := cfg.Records

		if records.CNAME != "" {
			cnameRR := &dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   subdomainFqdn,
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    ttl,
				},
				Target: dns.Fqdn(records.CNAME),
			}
			sb.WriteString(cnameRR.String() + "\n")
		}

		for _, ipStr := range records.A {
			aRR := &dns.A{
				Hdr: dns.RR_Header{
					Name:   subdomainFqdn,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    ttl,
				},
				A: net.ParseIP(ipStr),
			}
			sb.WriteString(aRR.String() + "\n")
		}

		for _, ipStr := range records.AAAA {
			aaaaRR := &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   subdomainFqdn,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    ttl,
				},
				AAAA: net.ParseIP(ipStr),
			}
			sb.WriteString(aaaaRR.String() + "\n")
		}

		for _, txtStr := range records.TXT {
			txtRR := &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   subdomainFqdn,
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    ttl,
				},
				Txt: []string{txtStr},
			}
			sb.WriteString(txtRR.String() + "\n")
		}

		for _, nsTarget := range records.NS {
			nsRR := &dns.NS{
				Hdr: dns.RR_Header{
					Name:   subdomainFqdn,
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    ttl,
				},
				Ns: dns.Fqdn(nsTarget),
			}
			sb.WriteString(nsRR.String() + "\n")
		}

		if records.AcmeChallenge != "" {
			acmeFqdn := dns.Fqdn(fmt.Sprintf("_acme-challenge.%s.%s", cfg.Subdomain, zone))
			acmeRR := &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   acmeFqdn,
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				Txt: []string{records.AcmeChallenge},
			}
			sb.WriteString(acmeRR.String() + "\n")
		}
	}

	return sb.String(), nil
}

func (c *Compiler) VerifyZone(zone, zoneContent string) error {
	origin := dns.Fqdn(zone)
	reader := strings.NewReader(zoneContent)
	zp := dns.NewZoneParser(reader, origin, "")

	hasSOA := false
	rrCount := 0

	for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
		if rr.Header().Rrtype == dns.TypeSOA {
			hasSOA = true
		}
		rrCount++
	}

	if err := zp.Err(); err != nil {
		return fmt.Errorf("zone syntax error in %s: %w", zone, err)
	}

	if !hasSOA {
		return fmt.Errorf("zone %s has no SOA record", zone)
	}
	if rrCount < 4 {
		return fmt.Errorf("zone %s has insufficient records (%d)", zone, rrCount)
	}

	return nil
}
