package config

// Acme holds the ACME (Let's Encrypt) configuration. Each challenge type has
// its own table; the table you configure determines the challenge. The
// collection is keyed by an arbitrary user-chosen label — see AGENTS.md
// "Config: map key rules". The engine that consumes this shape lives in
// restinpieces-acme; the framework only hosts the shape and its validation.
type Acme struct {
	// Account is the ACME account identity, shared by every challenge type.
	Account AcmeAccount `toml:"account" comment:"ACME account identity"`

	// Domains lists every name the certificate must cover. For a wildcard,
	// include the base domain and the wildcard: ["example.com", "*.example.com"].
	// Entries may also be IP addresses, for example "192.0.2.1" or
	// "2001:db8::1"; they are matched against the addresses in the
	// certificate.
	// Names may use Unicode (IDNA); they are converted to their ASCII
	// (punycode) form when the certificate is requested and when the staged
	// certificate is compared with this list.
	Domains []string `toml:"domains" comment:"Names and IPs the certificate must cover"`

	// CADirectoryURL is the ACME server. Staging and production are separate
	// accounts:
	//   https://acme-staging-v02.api.letsencrypt.org/directory
	//   https://acme-v02.api.letsencrypt.org/directory
	CADirectoryURL string `toml:"ca_directory_url" comment:"ACME server directory URL"`

	// Profile selects the ACME profile that shapes the issued certificate,
	// for example its lifetime. An empty value lets the CA pick its default
	// profile, which is the right choice for most subscribers. Profile names
	// are specific to the CA and are listed in its ACME directory; Let's
	// Encrypt currently offers "classic" (90 days), "tlsserver" (45 days) and
	// "shortlived" (about 6 days). Profiles are defined by
	// draft-ietf-acme-profiles, so a CA may not support them at all.
	Profile string `toml:"profile" comment:"ACME profile name; empty lets the CA choose"`

	// RemainingLifetimeFraction is the share of the certificate's lifetime
	// that may remain before a new certificate is requested (0.25 = act when
	// a quarter of the lifetime is left). The handler requests a new
	// certificate when the CA asks for one or when the remaining lifetime
	// drops to this share.
	RemainingLifetimeFraction float64 `toml:"remaining_lifetime_fraction" comment:"Renew when this share of the lifetime remains"`

	// DNS01 holds the dns-01 challenge entries.
	DNS01 AcmeDNS01 `toml:"dns-01" comment:"dns-01 challenge settings"`

	// Certificate is the PEM certificate chain staged by the engine.
	Certificate string `toml:"certificate" comment:"Staged PEM certificate chain"`

	// PrivateKey is the PEM private key matching Certificate.
	PrivateKey string `toml:"private_key" comment:"Staged PEM private key"`
}

// AcmeAccount is the ACME account identity, shared by all challenge types.
type AcmeAccount struct {
	// Email is the account contact address. It must be at a domain you control;
	// Let's Encrypt rejects reserved domains such as example.com.
	Email string `toml:"email" comment:"Account contact email"`

	// Key is the account private key in PEM form. lego accepts ECDSA (P-256 or
	// P-384) and RSA; Ed25519 is not supported. It identifies the account, so
	// keep it secret and reuse it.
	Key string `toml:"key" comment:"PEM account private key (ECDSA or RSA)"`
}

// AcmeDNS01 holds the dns-01 challenge entries. The map key is an arbitrary
// user-chosen label — see AGENTS.md "Config: map key rules".
type AcmeDNS01 map[string]AcmeDNS01Entry

// AcmeDNS01Entry is one dns-01 entry.
//
// Provider names the DNS implementation, for example "cloudflare"; an empty
// provider deactivates the entry. Credentials holds whatever fields that
// implementation needs — the framework does not know their names, the engine
// does.
type AcmeDNS01Entry struct {
	Provider    string            `toml:"provider" comment:"DNS provider implementation (e.g. 'cloudflare'). Empty deactivates this entry."`
	Credentials map[string]string `toml:"credentials" comment:"Provider-specific credentials."`
}

func (c Config) AcmeDNS01() AcmeDNS01 {
	return c.Acme.DNS01
}
