# be-a.dev ⚡

[![be-a.dev Status](https://img.shields.io/badge/status-active-10b981?style=flat-square)](https://be-a.dev)
[![Go Version](https://img.shields.io/badge/go-1.26+-06b6d4?style=flat-square&logo=go)](https://golang.org)
[![CoreDNS](https://img.shields.io/badge/dns-CoreDNS%201.12-8b5cf6?style=flat-square)](https://coredns.io)
[![RFC Compliant](https://img.shields.io/badge/rfc-1035%20%7C%201982%20%7C%202308-3b82f6?style=flat-square)](https://www.ietf.org/standards/rfcs/)
[![License](https://img.shields.io/badge/license-MIT-gray?style=flat-square)](LICENSE)

> Minimal, ultra-fast, and free subdomains for developers and makers.  
> Powered by GitOps, RFC-compliant DNS, and an authoritative CoreDNS cluster distributed across Europe.

🌐 **Web Portal & Live Search:** [https://be-a.dev](https://be-a.dev)

---

```
$ dig +noall +answer d3vo.be-a.dev @ns1.be-a.dev
d3vo.be-a.dev.        300    IN    CNAME    d3vo.ru.
```

---

## ⚡ Quickstart: Claim Your Subdomain

### Option A: 1-Click via Web Portal (Recommended)
1. Open the [be-a.dev Web Portal](https://be-a.dev).
2. Enter your handle or project name (e.g. `alex`, `my-bot`, `api`).
3. Click **Claim Subdomain**, fill in your GitHub username and target host.
4. Click **🚀 Open GitHub Pull Request** — GitHub opens a pre-filled PR ready to merge.

### Option B: GitOps Pull Request
Fork this repository and add your domain JSON definition following single-character sharding:
`domains/<zone>/<first-letter>/<subdomain>.json`

**Example: `domains/be-a.dev/a/alex.json`:**
```json
{
  "$schema": "../../../schemas/domain.schema.json",
  "description": "Alex's personal portfolio & blog",
  "owner": {
    "username": "alex"
  },
  "records": {
    "CNAME": "alex.github.io."
  },
  "ttl": 300
}
```

---

## 📋 Supported Records & Validation Rules

| Record | Purpose | Constraints & Security |
| :--- | :--- | :--- |
| **`CNAME`** | GitHub Pages, Vercel, Cloudflare, Fly.io | **Exclusive:** Cannot coexist with A, AAAA or NS (RFC 1034 §3.6.2). Trailing dot normalized. |
| **`A`** | Dedicated server / VPS IPv4 | Public IPv4 only. Private ranges (RFC 1918 `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) and loopback blocked. |
| **`AAAA`** | VPS / Edge IPv6 | Public IPv6 only. Loopback `::1` and link-local `fe80::/10` blocked. |
| **`TXT`** | Domain verification (Google, Bluesky, etc.) | Max 255 characters per string. Coexists with A/AAAA. |
| **`NS`** | Subzone delegation | Minimum 2 nameservers (RFC 2182). Out-of-bailiwick only. |
| **`_acme-challenge`** | Let's Encrypt automated SSL validation | Permitted alongside CNAME for automatic Wildcard TLS (RFC 8555). |

---

## 🏛️ Authoritative Geo-DNS Cluster

Our DNS records are served directly by an authoritative CoreDNS DaemonSet bound to public edge nodes:

| Nameserver | Location | IPv4 Address | Latency |
| :--- | :--- | :--- | :--- |
| **`ns1.be-a.dev`** | 🇳🇱 Amsterdam, Netherlands | `130.78.191.155` | `<10ms` (EU West) |
| **`ns2.be-a.dev`** | 🇩🇪 Frankfurt, Germany | `191.44.118.80` | `<12ms` (EU Central) |
| **`ns3.be-a.dev`** | 🇵🇱 Warsaw, Poland | `222.167.248.169` | `<8ms` (EU East) |

### Engineering Highlights
* **RFC 1982:** Monotonic serial formatting (`YYYYMMDDnn`) updated on every merge.
* **RFC 2308:** 300-second negative caching TTL ensures newly claimed subdomains propagate globally within 5 minutes.
* **Wildcard Fallback:** Any unregistered query resolves to the portal, allowing visitors to instantly claim unclaimed names.
* **Automated RDAP Monitoring:** Daily health checks on root domain expiration with Prometheus metrics exports.

---

## 🛠️ Local CLI Tooling

The registry is managed via a zero-dependency Go CLI tool:

```bash
# Validate domain definitions against RFC rules and reserved stoplists
go run ./cmd/registry validate

# Compile all JSON records into RFC 1035 BIND zone files
go run ./cmd/registry compile --out-dir ./zones

# Run the web portal & live search server locally
go run ./cmd/registry serve --addr :8080

# Check live domain expiration via RDAP
go run ./cmd/registry expiry
```

---

## 🧪 Automated Testing

```bash
go test -v -count=1 ./...
```

---

## 📄 License

MIT © 2026 [xdearboy](https://github.com/xdearboy). Free for open-source and individual developers.
