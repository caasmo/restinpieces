package crypto

import (
	"strings"
	"testing"
)

func TestOauth2State(t *testing.T) {
	state := Oauth2State()
	if len(state) != Oauth2StateLength {
		t.Errorf("Oauth2State() length = %d, want %d", len(state), Oauth2StateLength)
	}
	for _, char := range state {
		if !strings.ContainsRune(AlphanumericAlphabet, char) {
			t.Errorf("Oauth2State() contains invalid character: %c", char)
		}
	}
}

func TestOauth2CodeVerifier(t *testing.T) {
	verifier := Oauth2CodeVerifier()
	if len(verifier) != OauthCodeVerifierLength {
		t.Errorf("Oauth2CodeVerifier() length = %d, want %d", len(verifier), OauthCodeVerifierLength)
	}
	for _, char := range verifier {
		if !strings.ContainsRune(pkceAlphabet, char) {
			t.Errorf("Oauth2CodeVerifier() contains invalid character: %c", char)
		}
	}
}

func TestS256Challenge(t *testing.T) {
	// Example from RFC 7636
	code := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	expectedChallenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	challenge := S256Challenge(code)
	if challenge != expectedChallenge {
		t.Errorf("S256Challenge() = %s, want %s", challenge, expectedChallenge)
	}
}