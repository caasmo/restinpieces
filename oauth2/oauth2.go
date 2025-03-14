package oauth2

import (
)

type oauth2Response struct {
	Providers []providerInfo `json:"providers"`
}

type providerInfo struct {
	Name               string `json:"name"`
	DisplayName        string `json:"displayName"`
	State              string `json:"state"`
	AuthURL            string `json:"authURL"`
	CodeVerifier       string `json:"codeVerifier,omitempty"`
	CodeChallenge      string `json:"codeChallenge,omitempty"`
	CodeChallengeMethod string `json:"codeChallengeMethod,omitempty"`
}

