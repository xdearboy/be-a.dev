package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xdearboy/free-domains/pkg/compiler"
	"github.com/xdearboy/free-domains/pkg/expiry"
	"github.com/xdearboy/free-domains/pkg/models"
	"github.com/xdearboy/free-domains/pkg/validator"
)

//go:embed static/*
var staticFS embed.FS

type CheckResult struct {
	Zone    string `json:"zone"`
	FQDN    string `json:"fqdn"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type CheckResponse struct {
	Subdomain string        `json:"subdomain"`
	Results   []CheckResult `json:"results"`
}

type NameserverStatus struct {
	Host   string `json:"host"`
	Region string `json:"region"`
	IP     string `json:"ip"`
	Status string `json:"status"`
}

type StatusResponse struct {
	PrimaryZone string                `json:"primary_zone"`
	Nameservers []NameserverStatus    `json:"nameservers"`
	Expiry      []expiry.DomainExpiry `json:"expiry"`
	TotalZones  int                   `json:"total_zones"`
	Timestamp   string                `json:"timestamp"`
}

type FallbackData struct {
	Subdomain string
	Zone      string
	Shard     string
}

type Server struct {
	domainsDir    string
	validator     *validator.Validator
	primaryZones  []string
	fallbackTmpl  *template.Template
	expiryChecker *expiry.Checker
}

func NewServer(domainsDir string, v *validator.Validator) (*Server, error) {
	raw, err := staticFS.ReadFile("static/fallback.html")
	if err != nil {
		return nil, fmt.Errorf("read fallback template: %w", err)
	}

	tmpl, err := template.New("fallback").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse fallback template: %w", err)
	}

	return &Server{
		domainsDir:    domainsDir,
		validator:     v,
		primaryZones:  []string{"be-a.dev"},
		fallbackTmpl:  tmpl,
		expiryChecker: expiry.NewChecker(6 * time.Hour),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("/api/check", s.handleCheck)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/", s.handleRoot)

	return mux
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	host := strings.ToLower(r.Host)
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	for _, zone := range s.primaryZones {
		suffix := "." + zone
		if !strings.HasSuffix(host, suffix) {
			continue
		}

		sub := strings.TrimSuffix(host, suffix)
		if sub == "" || sub == "www" {
			continue
		}

		shard := string(sub[0])
		jsonPath := filepath.Join(s.domainsDir, zone, shard, fmt.Sprintf("%s.json", sub))

		if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_ = s.fallbackTmpl.Execute(w, FallbackData{
				Subdomain: sub,
				Zone:      zone,
				Shard:     shard,
			})
			return
		}
	}

	indexRaw, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexRaw)
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("name")))
	w.Header().Set("Content-Type", "application/json")

	if name == "" {
		_ = json.NewEncoder(w).Encode(CheckResponse{Results: []CheckResult{}})
		return
	}

	resp := CheckResponse{
		Subdomain: name,
		Results:   make([]CheckResult, 0, len(s.primaryZones)),
	}

	for _, zone := range s.primaryZones {
		res := CheckResult{
			Zone: zone,
			FQDN: fmt.Sprintf("%s.%s", name, zone),
		}

		if len(name) < 1 || len(name) > 63 {
			res.Status = "invalid"
			res.Message = "Length must be between 1 and 63 characters"
			resp.Results = append(resp.Results, res)
			continue
		}

		err := s.validator.ValidateConfig(&models.DomainConfig{
			Subdomain: name,
			Zone:      zone,
			Owner:     models.Owner{Username: "probe"},
			Records:   models.RecordSet{A: []string{"1.1.1.1"}},
		}, "probe")

		if err != nil {
			msg := err.Error()
			switch {
			case strings.Contains(msg, "blacklisted"):
				res.Status = "blacklisted"
				res.Message = "Protected brand, payment, or system name"
			case strings.Contains(msg, "reserved"):
				res.Status = "reserved"
				res.Message = "Reserved for infrastructure endpoint"
			case strings.Contains(msg, "LDH") || strings.Contains(msg, "lowercase"):
				res.Status = "invalid"
				res.Message = "Only lowercase letters, numbers, and middle hyphens are allowed"
			default:
				res.Status = "restricted"
				res.Message = msg
			}
			resp.Results = append(resp.Results, res)
			continue
		}

		shard := string(name[0])
		jsonPath := filepath.Join(s.domainsDir, zone, shard, fmt.Sprintf("%s.json", name))
		if _, err := os.Stat(jsonPath); err == nil {
			res.Status = "taken"
			res.Message = "Already registered by another user"
		} else {
			res.Status = "available"
			res.Message = "Available for instant claim"
		}

		resp.Results = append(resp.Results, res)
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	regions := map[string]string{
		"ns1": "Netherlands (NL)",
		"ns2": "Germany (DE)",
		"ns3": "Poland (PL)",
	}

	nsList := make([]NameserverStatus, 0, len(compiler.DefaultNameservers))
	for _, ns := range compiler.DefaultNameservers {
		nsList = append(nsList, NameserverStatus{
			Host:   ns.Host,
			Region: regions[ns.Host],
			IP:     ns.IPv4,
			Status: "healthy",
		})
	}

	resp := StatusResponse{
		PrimaryZone: "be-a.dev",
		Nameservers: nsList,
		Expiry:      s.expiryChecker.CheckMultiple(s.primaryZones),
		TotalZones:  len(s.primaryZones),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	expiryList := s.expiryChecker.CheckMultiple(s.primaryZones)

	var sb strings.Builder
	sb.WriteString("# HELP domain_expiry_days_remaining Days remaining until parent domain expiration\n")
	sb.WriteString("# TYPE domain_expiry_days_remaining gauge\n")
	for _, exp := range expiryList {
		if !exp.ExpiresAt.IsZero() {
			sb.WriteString(fmt.Sprintf("domain_expiry_days_remaining{domain=%q} %d\n", exp.Domain, exp.DaysRemaining))
		}
	}

	sb.WriteString("\n# HELP domain_expiry_timestamp_seconds Unix timestamp of parent domain expiration\n")
	sb.WriteString("# TYPE domain_expiry_timestamp_seconds gauge\n")
	for _, exp := range expiryList {
		if !exp.ExpiresAt.IsZero() {
			sb.WriteString(fmt.Sprintf("domain_expiry_timestamp_seconds{domain=%q} %d\n", exp.Domain, exp.ExpiresAt.Unix()))
		}
	}

	_, _ = w.Write([]byte(sb.String()))
}
