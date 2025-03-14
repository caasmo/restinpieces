package oauth2

import (
)

type oauth2Response struct {
	Providers []providerInfo `json:"providers"`
}

type providerInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`
	AuthURL     string `json:"authURL"`

	// technically could be omitted if the provider doesn't support PKCE,
	// but to avoid breaking existing typed clients we'll return them as empty string
	//CodeVerifier        string `json:"codeVerifier"`
	//CodeChallenge       string `json:"codeChallenge"`
	//CodeChallengeMethod string `json:"codeChallengeMethod"`
}
