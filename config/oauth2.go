package config

const (
	OAuth2Google = "google"

	// OAuth2GitHub is a known provider name the framework does not map yet.
	// validateOAuth2 rejects it until a mapping is added to
	// oauth2.UserFromUserInfo.
	OAuth2GitHub = "github"
)

// OAuth2Entry describes one OAuth2 provider configuration.
type OAuth2Entry struct {
	Name         string `toml:"name" comment:"Provider identifier the login endpoint dispatches on (e.g. 'google')"`
	ClientID     string `toml:"client_id" comment:"OAuth2 client ID (set via env)"`
	ClientSecret string `toml:"client_secret" comment:"OAuth2 client secret (set via env)"`
	DisplayName  string `toml:"display_name" comment:"User-facing provider name"`
	// RedirectURLPath is the callback path the provider sends the visitor back
	// to, for example "/oauth2/google/callback". The complete callback address
	// is this path added to server.public_url.
	RedirectURLPath string   `toml:"redirect_url_path" comment:"Callback path added to server.public_url (e.g. '/oauth2/google/callback')"`
	AuthURL         string   `toml:"auth_url" comment:"OAuth2 authorization endpoint"`
	TokenURL        string   `toml:"token_url" comment:"OAuth2 token endpoint"`
	UserInfoURL     string   `toml:"user_info_url" comment:"User info API endpoint"`
	Scopes          []string `toml:"scopes" comment:"Requested OAuth2 scopes"`
	PKCE            bool     `toml:"pkce" comment:"Enable PKCE flow"`
}

// OAuth2 lists the configured providers. Each map key is a label you choose,
// for example "my_google". The provider identifier is OAuth2Entry.Name, not
// the key.
type OAuth2 map[string]OAuth2Entry

// Get returns the entry whose Name matches label. The second result is false
// when no entry has that name. Two entries must not use the same name
// (validateOAuth2 rejects that), so the first match is the only match.
func (o OAuth2) Get(label string) (OAuth2Entry, bool) {
	for _, entry := range o {
		if entry.Name == label {
			return entry, true
		}
	}
	return OAuth2Entry{}, false
}
