package models

type Owner struct {
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
}

type RecordSet struct {
	A             []string `json:"A,omitempty"`
	AAAA          []string `json:"AAAA,omitempty"`
	CNAME         string   `json:"CNAME,omitempty"`
	TXT           []string `json:"TXT,omitempty"`
	NS            []string `json:"NS,omitempty"`
	AcmeChallenge string   `json:"_acme-challenge,omitempty"`
}

type DomainConfig struct {
	Schema      string    `json:"$schema,omitempty"`
	Description string    `json:"description,omitempty"`
	Owner       Owner     `json:"owner"`
	Records     RecordSet `json:"records"`
	TTL         int       `json:"ttl,omitempty"`

	Subdomain string `json:"-"`
	Zone      string `json:"-"`
	FilePath  string `json:"-"`
}

type BlacklistConfig struct {
	Exact    []string `json:"exact"`
	Patterns []string `json:"patterns"`
}

type ReservedConfig map[string][]string
