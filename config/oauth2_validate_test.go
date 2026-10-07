package config

import "testing"

func TestValidateOAuth2(t *testing.T) {
	t.Parallel()
	validCases := []OAuth2{
		{"google": {Name: "google", RedirectURLPath: "/cb"}},
		{"my_google": {Name: "google", RedirectURLPath: "/oauth2/google/callback"}},
		{"my_google": {Name: "google", RedirectURLPath: "/cb", AuthURL: "https://accounts.example.com/auth", TokenURL: "https://accounts.example.com/token", UserInfoURL: "https://accounts.example.com/userinfo"}},
	}
	for _, cfg := range validCases {
		if err := validateOAuth2(cfg); err != nil {
			t.Errorf("validateOAuth2(%+v) failed: %v", cfg, err)
		}
	}

	invalidCases := []OAuth2{
		{"google": {Name: "google"}},
		{"google": {Name: "google", RedirectURLPath: "cb"}},
		{"google": {Name: "google", RedirectURLPath: "//example.com/cb"}},
		{"google": {Name: "google", RedirectURLPath: "/cb/"}},
		{"google": {Name: "google", RedirectURLPath: "/"}},
		{"google": {Name: "google", RedirectURLPath: "/cb", AuthURL: "http://example.com/auth"}},
		{"google": {Name: "google", RedirectURLPath: "/cb", TokenURL: "http://example.com/token"}},
		{"google": {Name: "google", RedirectURLPath: "/cb", UserInfoURL: "http://example.com"}},
		{"my_google": {RedirectURLPath: "/cb"}},
		{"my_github": {Name: OAuth2GitHub, RedirectURLPath: "/cb"}},
		{"my google": {Name: "google", RedirectURLPath: "/cb"}},
		{"my.google": {Name: "google", RedirectURLPath: "/cb"}},
		{"first": {Name: "google", RedirectURLPath: "/a"}, "second": {Name: "google", RedirectURLPath: "/b"}},
	}
	for _, cfg := range invalidCases {
		if err := validateOAuth2(cfg); err == nil {
			t.Errorf("validateOAuth2(%+v) expected error, got nil", cfg)
		}
	}
}
