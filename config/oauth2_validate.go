package config

import (
	"fmt"
	"strings"
)

// validateOAuth2 checks every entry in the OAuth2 section.
//
// A map key is a label you choose; the provider identifier lives in
// OAuth2Entry.Name. The name must be set, unique, and supported: the
// user-info mapping in oauth2.UserFromUserInfo implements google only, so
// an unsupported name is rejected here instead of failing after the token
// exchange has burned the authorization code.
func validateOAuth2(oauth2 OAuth2) error {
	allowedNames := map[string]struct{}{
		OAuth2Google: {},
	}
	entryNames := make(map[string]string, len(oauth2))
	for label, entry := range oauth2 {
		if !isValidMapKeyLabel(label) {
			return fmt.Errorf("oauth2: map key %q must not contain whitespace or '.'", label)
		}
		if entry.Name == "" {
			return fmt.Errorf("oauth2 entry '%s' must have name configured", label)
		}
		if _, ok := allowedNames[entry.Name]; !ok {
			return fmt.Errorf("oauth2 entry '%s' name %q is not supported (supported: %s)", label, entry.Name, OAuth2Google)
		}
		if other, ok := entryNames[entry.Name]; ok {
			return fmt.Errorf("oauth2 entry '%s' name %q is already used by entry '%s'", label, entry.Name, other)
		}
		entryNames[entry.Name] = label
		if entry.RedirectURLPath == "" {
			return fmt.Errorf("oauth2 entry '%s' must have redirect_url_path configured", label)
		}
		if !strings.HasPrefix(entry.RedirectURLPath, "/") || strings.HasPrefix(entry.RedirectURLPath, "//") {
			return fmt.Errorf("oauth2 entry '%s' redirect_url_path '%s' must start with a single '/'", label, entry.RedirectURLPath)
		}
		if strings.HasSuffix(entry.RedirectURLPath, "/") {
			return fmt.Errorf("oauth2 entry '%s' redirect_url_path '%s' must not end with '/'", label, entry.RedirectURLPath)
		}
		if entry.AuthURL != "" && !strings.HasPrefix(entry.AuthURL, "https://") {
			return fmt.Errorf("oauth2 entry '%s' AuthURL must use HTTPS: %s", label, entry.AuthURL)
		}
		if entry.TokenURL != "" && !strings.HasPrefix(entry.TokenURL, "https://") {
			return fmt.Errorf("oauth2 entry '%s' TokenURL must use HTTPS: %s", label, entry.TokenURL)
		}
		if entry.UserInfoURL != "" && !strings.HasPrefix(entry.UserInfoURL, "https://") {
			return fmt.Errorf("oauth2 entry '%s' UserInfoURL must use HTTPS: %s", label, entry.UserInfoURL)
		}
	}
	return nil
}
