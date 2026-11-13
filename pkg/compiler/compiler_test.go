package compiler

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xdearboy/free-domains/pkg/models"
	"github.com/xdearboy/free-domains/pkg/validator"
)

func TestCompilerGeneration(t *testing.T) {
	v, err := validator.NewValidator(
		filepath.Join("..", "..", "config", "blacklist.json"),
		filepath.Join("..", "..", "config", "reserved.json"),
	)
	if err != nil {
		t.Fatalf("failed to init validator: %v", err)
	}

	comp := NewCompiler(v)

	configs := []*models.DomainConfig{
		{
			Subdomain: "alice",
			Zone:      "kra67.space",
			Owner:     models.Owner{Username: "alice"},
			Records: models.RecordSet{
				CNAME:         "alice.github.io",
				AcmeChallenge: "token-123456",
			},
			TTL: 300,
		},
		{
			Subdomain: "demo",
			Zone:      "kra67.space",
			Owner:     models.Owner{Username: "demo"},
			Records: models.RecordSet{
				A:    []string{"185.12.34.56"},
				AAAA: []string{"2a00:1450:4001:828::200e"},
				TXT:  []string{"v=spf1 ..."},
			},
			TTL: 600,
		},
		{
			Subdomain: "sub",
			Zone:      "kra67.space",
			Owner:     models.Owner{Username: "subuser"},
			Records: models.RecordSet{
				NS: []string{"ns1.he.net", "ns2.he.net"},
			},
			TTL: 86400,
		},
	}

	zoneText, err := comp.GenerateZone("kra67.space", configs)
	if err != nil {
		t.Fatalf("failed to generate zone: %v", err)
	}

	// Verify zone text
	if !strings.Contains(zoneText, "SOA") {
		t.Errorf("zone text missing SOA record")
	}
	if !strings.Contains(zoneText, "ns1.kra67.space.") {
		t.Errorf("zone text missing primary NS record")
	}
	if !strings.Contains(zoneText, "alice.kra67.space.	300	IN	CNAME	alice.github.io.") {
		t.Errorf("zone text missing alice CNAME record with trailing dot, got:\n%s", zoneText)
	}
	if !strings.Contains(zoneText, "_acme-challenge.alice.kra67.space.	60	IN	TXT	\"token-123456\"") {
		t.Errorf("zone text missing _acme-challenge TXT record, got:\n%s", zoneText)
	}

	// Verify using miekg/dns parser
	if err := comp.VerifyZone("kra67.space", zoneText); err != nil {
		t.Fatalf("zone verification failed: %v", err)
	}
}

func TestCompileDirectory(t *testing.T) {
	v, err := validator.NewValidator(
		filepath.Join("..", "..", "config", "blacklist.json"),
		filepath.Join("..", "..", "config", "reserved.json"),
	)
	if err != nil {
		t.Fatalf("failed to init validator: %v", err)
	}

	comp := NewCompiler(v)
	tmpOut := t.TempDir()

	results, err := comp.CompileDirectory(filepath.Join("..", "..", "domains"), tmpOut)
	if err != nil {
		t.Fatalf("failed to compile directory: %v", err)
	}

	if len(results) == 0 {
		t.Errorf("expected at least 1 zone compiled, got 0")
	}

	if count, ok := results["kra67.space"]; !ok || count < 2 {
		t.Errorf("expected kra67.space to have at least 2 domains, got %d", count)
	}
}
