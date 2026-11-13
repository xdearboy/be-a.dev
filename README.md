# FreeDomains Registry 🌐

[![RFC Compliance](https://img.shields.io/badge/RFC-1035%20%7C%201982%20%7C%202308%20%7C%208482-06b6d4)](https://www.ietf.org/standards/rfcs/)
[![Go Version](https://img.shields.io/badge/Go-1.26+-3b82f6.svg)](https://golang.org)
[![CoreDNS](https://img.shields.io/badge/DNS-CoreDNS%20Authoritative-8b5cf6)](https://coredns.io)

Open-source, GitOps-driven free subdomain registry for developers and creators. Get your free subdomain under **`kra67.space`** or **`plmt.tech`** powered by a high-performance authoritative CoreDNS cluster distributed across European datacenters.

---

## 🚀 How to Claim a Free Subdomain

### Method 1: Web Portal (1-Click PR)
1. Open the [FreeDomains Web Portal](http://localhost:8080) (or visit your production URL).
2. Enter your desired subdomain in the search bar.
3. If available, click **Claim**, fill in your GitHub username and target CNAME or IP.
4. Click **🚀 Open GitHub Pull Request** — GitHub opens a pre-filled PR ready to submit!

### Method 2: Manual Pull Request
Fork this repository and add your domain JSON definition following 1-character sharding:
`domains/<zone>/<first-letter>/<subdomain>.json`

**Example: `domains/kra67.space/a/alice.json`:**
```json
{
  "$schema": "../../../schemas/domain.schema.json",
  "description": "Alice's personal dev blog and API",
  "owner": {
    "username": "alice"
  },
  "records": {
    "CNAME": "alice.github.io."
  },
  "ttl": 300
}
```

---

## 📜 Supported Records & RFC Rules

| Type | Description | RFC Rules |
| :--- | :--- | :--- |
| **`CNAME`** | Alias to external host (GitHub Pages, Vercel) | **Exclusive:** Cannot coexist with A, AAAA, TXT or NS (RFC 1034 §3.6.2). Trailing dot normalized. |
| **`A`** | IPv4 address | Public IPv4 only. Private & loopback subnets blocked (RFC 1918). |
| **`AAAA`** | IPv6 address | Public IPv6 only. Loopback `::1` and link-local blocked. |
| **`TXT`** | Verification tokens (Google, etc.) | Maximum 255 characters per string. |
| **`NS`** | Subzone delegation | Minimum 2 nameservers (RFC 2182). Out-of-bailiwick only. |
| **`_acme-challenge`** | Let's Encrypt SSL verification token | Permitted alongside CNAME for automated Wildcard SSL (RFC 8555). |

---

## 🏛️ Authoritative Nameserver Cluster

The zones are served directly by an authoritative CoreDNS DaemonSet distributed across European nodes:

* 🇳🇱 **ns1.kra67.space** (`130.78.191.155` - Netherlands)
* 🇩🇪 **ns2.kra67.space** (`191.44.118.80` - Germany)
* 🇵🇱 **ns3.kra67.space** (`222.167.248.169` - Poland)

### ✨ CoreDNS Highlights:
* **RFC 1982:** Incremental serial arithmetic (`YYYYMMDDnn`).
* **RFC 2308:** 5-minute negative caching TTL (`300s`) so freshly created domains resolve globally without 24h caching delays.
* **RFC 8482:** Mitigation against DNS amplification DDoS attacks on `ANY` queries (`minimal-any`).
* **Catch-All Wildcard Fallback:** Any unallocated subdomain queries resolve to the web portal landing page inviting visitors to claim the name.

---

## 🛠️ CLI Tooling

The repository includes a unified Go binary (`cmd/registry`):

```bash
# 1. Validate all definitions against RFC standards & blacklists
./bin/registry validate

# 2. Compile JSON files into RFC 1035 BIND zone files
./bin/registry compile --out-dir ./zones

# 3. Start the Web Portal, Live Search API & Fallback server
./bin/registry serve --addr :8080
```

---

## 🧪 Testing

```bash
# Run unit & integration tests
go test -v ./...
```
