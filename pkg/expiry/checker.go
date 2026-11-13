package expiry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ErrNoExpirationEvent = errors.New("no expiration event in rdap response")
	ErrWhoisParseFailed  = errors.New("cannot locate expiry in whois response")

	whoisExpiryPattern = regexp.MustCompile(`(?i)(?:Registry Expiry Date|paid-till|Expiration Date|Expiry Date):\s*([0-9TZ:.\-]+)`)
)

type DomainExpiry struct {
	Domain        string    `json:"domain"`
	ExpiresAt     time.Time `json:"expires_at"`
	DaysRemaining int       `json:"days_remaining"`
	Status        string    `json:"status"`
	Source        string    `json:"source"`
	LastChecked   time.Time `json:"last_checked"`
}

type cachedEntry struct {
	data      DomainExpiry
	expiresAt time.Time
}

type Checker struct {
	httpClient *http.Client
	cacheTTL   time.Duration
	cache      map[string]cachedEntry
	mu         sync.RWMutex
}

func NewChecker(cacheTTL time.Duration) *Checker {
	if cacheTTL <= 0 {
		cacheTTL = 6 * time.Hour
	}
	return &Checker{
		httpClient: &http.Client{Timeout: 8 * time.Second},
		cacheTTL:   cacheTTL,
		cache:      make(map[string]cachedEntry),
	}
}

func (c *Checker) CheckDomain(domain string) (DomainExpiry, error) {
	norm := strings.ToLower(strings.TrimSpace(domain))

	c.mu.RLock()
	if entry, ok := c.cache[norm]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.data, nil
	}
	c.mu.RUnlock()

	expTime, err := c.queryRDAP(norm)
	source := "rdap"

	if err != nil {
		expTime, err = c.queryWHOIS(norm)
		source = "whois"
		if err != nil {
			return DomainExpiry{}, fmt.Errorf("%s: %w", norm, err)
		}
	}

	daysRemaining := int(time.Until(expTime).Hours() / 24)
	status := "healthy"
	if daysRemaining <= 0 {
		status = "expired"
	} else if daysRemaining <= 14 {
		status = "critical"
	} else if daysRemaining <= 45 {
		status = "warning"
	}

	result := DomainExpiry{
		Domain:        norm,
		ExpiresAt:     expTime,
		DaysRemaining: daysRemaining,
		Status:        status,
		Source:        source,
		LastChecked:   time.Now().UTC(),
	}

	c.mu.Lock()
	c.cache[norm] = cachedEntry{
		data:      result,
		expiresAt: time.Now().Add(c.cacheTTL),
	}
	c.mu.Unlock()

	return result, nil
}

func (c *Checker) CheckMultiple(domains []string) []DomainExpiry {
	var wg sync.WaitGroup
	results := make([]DomainExpiry, len(domains))

	for i, d := range domains {
		wg.Add(1)
		go func(idx int, dom string) {
			defer wg.Done()
			exp, err := c.CheckDomain(dom)
			if err != nil {
				results[idx] = DomainExpiry{
					Domain: dom,
					Status: "unknown",
				}
				return
			}
			results[idx] = exp
		}(i, d)
	}

	wg.Wait()
	return results
}

type rdapPayload struct {
	Events []struct {
		EventAction string `json:"eventAction"`
		EventDate   string `json:"eventDate"`
	} `json:"events"`
}

func (c *Checker) queryRDAP(domain string) (time.Time, error) {
	url := fmt.Sprintf("https://rdap.org/domain/%s", domain)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("User-Agent", "free-domains-registry/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("rdap status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return time.Time{}, err
	}

	var payload rdapPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return time.Time{}, err
	}

	for _, ev := range payload.Events {
		if ev.EventAction == "expiration" {
			if t, err := time.Parse(time.RFC3339, ev.EventDate); err == nil {
				return t, nil
			}
			if t, err := time.Parse("2006-01-02T15:04:05Z", ev.EventDate); err == nil {
				return t, nil
			}
		}
	}

	return time.Time{}, ErrNoExpirationEvent
}

func (c *Checker) queryWHOIS(domain string) (time.Time, error) {
	conn, err := net.DialTimeout("tcp", "whois.iana.org:43", 5*time.Second)
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()

	if _, err := fmt.Fprintf(conn, "%s\r\n", domain); err != nil {
		return time.Time{}, err
	}

	raw, err := io.ReadAll(conn)
	if err != nil {
		return time.Time{}, err
	}

	matches := whoisExpiryPattern.FindStringSubmatch(string(raw))
	if len(matches) < 2 {
		return time.Time{}, ErrWhoisParseFailed
	}

	dateStr := strings.TrimSpace(matches[1])
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported date format %q", dateStr)
}
