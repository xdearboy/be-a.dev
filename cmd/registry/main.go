package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/xdearboy/free-domains/pkg/compiler"
	"github.com/xdearboy/free-domains/pkg/expiry"
	"github.com/xdearboy/free-domains/pkg/server"
	"github.com/xdearboy/free-domains/pkg/validator"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: registry <command> [options]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  validate   Validate domain definitions against RFC rules and blacklists\n")
	fmt.Fprintf(os.Stderr, "  compile    Compile JSON definitions into RFC 1035 BIND zone files\n")
	fmt.Fprintf(os.Stderr, "  serve      Start web portal, search API, and wildcard fallback\n")
	fmt.Fprintf(os.Stderr, "  expiry     Check expiration status for parent domains\n\n")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "validate":
		runValidate(args)
	case "compile":
		runCompile(args)
	case "serve":
		runServe(args)
	case "expiry":
		runExpiry(args)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	domainsDir := fs.String("domains-dir", "./domains", "domains directory path")
	blacklistPath := fs.String("blacklist", "./config/blacklist.json", "blacklist config path")
	reservedPath := fs.String("reserved", "./config/reserved.json", "reserved config path")
	gitActor := fs.String("actor", "", "github pr author username")
	filesList := fs.String("files", "", "comma-separated files to validate")
	_ = fs.Parse(args)

	v, err := validator.NewValidator(*blacklistPath, *reservedPath)
	if err != nil {
		log.Fatalf("validator init: %v", err)
	}

	var targets []string

	if *filesList != "" {
		for _, f := range strings.Split(*filesList, ",") {
			trimmed := strings.TrimSpace(f)
			if trimmed != "" && strings.HasSuffix(trimmed, ".json") && strings.HasPrefix(trimmed, "domains/") {
				targets = append(targets, trimmed)
			}
		}
	} else {
		err := filepath.Walk(*domainsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
				targets = append(targets, path)
			}
			return nil
		})
		if err != nil {
			log.Fatalf("scan %s: %v", *domainsDir, err)
		}
	}

	if len(targets) == 0 {
		fmt.Println("No domain files to validate.")
		return
	}

	var errs int
	for _, path := range targets {
		cfg, err := v.ValidateFile(path, *gitActor)
		if err != nil {
			fmt.Printf("FAIL %s: %v\n", path, err)
			errs++
			continue
		}
		fmt.Printf("OK   %s.%s (%s)\n", cfg.Subdomain, cfg.Zone, path)
	}

	if errs > 0 {
		fmt.Printf("\nValidation failed: %d error(s)\n", errs)
		os.Exit(1)
	}

	fmt.Printf("\nSuccessfully validated %d file(s).\n", len(targets))
}

func runCompile(args []string) {
	fs := flag.NewFlagSet("compile", flag.ExitOnError)
	domainsDir := fs.String("domains-dir", "./domains", "domains directory path")
	outDir := fs.String("out-dir", "./zones", "output directory for zone files")
	blacklistPath := fs.String("blacklist", "./config/blacklist.json", "blacklist config path")
	reservedPath := fs.String("reserved", "./config/reserved.json", "reserved config path")
	_ = fs.Parse(args)

	v, err := validator.NewValidator(*blacklistPath, *reservedPath)
	if err != nil {
		log.Fatalf("validator init: %v", err)
	}

	c := compiler.NewCompiler(v)
	results, err := c.CompileDirectory(*domainsDir, *outDir)
	if err != nil {
		log.Fatalf("compile: %v", err)
	}

	var total int
	for zone, count := range results {
		fmt.Printf("zone %s: %d record(s) -> %s/db.%s\n", zone, count, *outDir, zone)
		total += count
	}

	fmt.Printf("Total: %d domain(s) compiled across %d zone(s)\n", total, len(results))
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	domainsDir := fs.String("domains-dir", "./domains", "domains directory path")
	addr := fs.String("addr", ":8080", "http bind address")
	blacklistPath := fs.String("blacklist", "./config/blacklist.json", "blacklist config path")
	reservedPath := fs.String("reserved", "./config/reserved.json", "reserved config path")
	_ = fs.Parse(args)

	v, err := validator.NewValidator(*blacklistPath, *reservedPath)
	if err != nil {
		log.Fatalf("validator init: %v", err)
	}

	srv, err := server.NewServer(*domainsDir, v)
	if err != nil {
		log.Fatalf("server init: %v", err)
	}

	log.Printf("server listening on %s (domains: %s)", *addr, *domainsDir)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func runExpiry(args []string) {
	fs := flag.NewFlagSet("expiry", flag.ExitOnError)
	zonesFlag := fs.String("zones", "kra67.space,plmt.tech", "comma-separated list of domains to query")
	_ = fs.Parse(args)

	domains := strings.Split(*zonesFlag, ",")
	chk := expiry.NewChecker(0)

	results := chk.CheckMultiple(domains)

	fmt.Printf("%-20s | %-24s | %-10s | %-10s\n", "DOMAIN", "EXPIRES (UTC)", "DAYS LEFT", "STATUS")
	fmt.Println(strings.Repeat("-", 72))

	for _, r := range results {
		dateStr := "N/A"
		if !r.ExpiresAt.IsZero() {
			dateStr = r.ExpiresAt.Format("2006-01-02 15:04 UTC")
		}
		fmt.Printf("%-20s | %-24s | %-10d | %-10s\n", r.Domain, dateStr, r.DaysRemaining, strings.ToUpper(r.Status))
	}
}
