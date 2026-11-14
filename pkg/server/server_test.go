package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xdearboy/free-domains/pkg/validator"
)

func createTestServer(t *testing.T) *Server {
	t.Helper()
	v, err := validator.NewValidator(
		filepath.Join("..", "..", "config", "blacklist.json"),
		filepath.Join("..", "..", "config", "reserved.json"),
	)
	if err != nil {
		t.Fatalf("failed to init validator: %v", err)
	}

	srv, err := NewServer(filepath.Join("..", "..", "domains"), v)
	if err != nil {
		t.Fatalf("failed to init server: %v", err)
	}
	return srv
}

func TestIndexPage(t *testing.T) {
	srv := createTestServer(t)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "be-a.dev") {
		t.Errorf("index page missing be-a.dev branding")
	}
}

func TestWildcardFallback(t *testing.T) {
	srv := createTestServer(t)

	// Simulate requesting an unallocated domain
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "unallocated-project-99.be-a.dev"
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "unallocated-project-99") || !strings.Contains(body, "be-a.dev") {
		t.Errorf("fallback page did not render domain name correctly, got:\n%s", body)
	}
}

func TestCheckAPI(t *testing.T) {
	srv := createTestServer(t)

	tests := []struct {
		name           string
		query          string
		expectedStatus map[string]string
	}{
		{
			name:  "Existing domain",
			query: "d3vo",
			expectedStatus: map[string]string{
				"be-a.dev": "taken",
			},
		},
		{
			name:  "Available domain",
			query: "my-brandnew-app",
			expectedStatus: map[string]string{
				"be-a.dev": "available",
			},
		},
		{
			name:  "Blacklisted domain",
			query: "sberbank",
			expectedStatus: map[string]string{
				"be-a.dev": "blacklisted",
			},
		},
		{
			name:  "Reserved domain on be-a.dev",
			query: "dashboard",
			expectedStatus: map[string]string{
				"be-a.dev": "reserved",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/check?name="+tc.query, nil)
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", w.Code)
			}

			var resp CheckResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse JSON response: %v", err)
			}

			for _, r := range resp.Results {
				expected, ok := tc.expectedStatus[r.Zone]
				if ok && r.Status != expected {
					t.Errorf("for zone %s expected status '%s', got '%s' (message: %s)",
						r.Zone, expected, r.Status, r.Message)
				}
			}
		})
	}
}

func TestStatusAPI(t *testing.T) {
	srv := createTestServer(t)

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp StatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse status JSON: %v", err)
	}

	if len(resp.Nameservers) != 3 {
		t.Errorf("expected 3 nameservers, got %d", len(resp.Nameservers))
	}
}
