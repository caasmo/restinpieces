package crypto

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// Defined in RFC 7636 (PKCE). Allowed characters: A-Z, a-z, 0-9, and the symbols -, ., _, ~.
const pkceAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"

// PKCE code challenge method as defined in RFC 7636
const PKCECodeChallengeMethod = "S256"


// The OAuth2 specification (RFC 6749) doesn’t mandate a specific length. It
// recommends a random, unguessable string.
// At least 16 characters, though 32 to 64 characters is common
// for better uniqueness and security.
const Oauth2StateLength = 32

// Defined in RFC 7636 (PKCE). Its length must be between 43 and 128 characters.
const OauthCodeVerifierLength = 43

// The state parameter helps prevent Cross-Site Request Forgery (CSRF) attacks
// by linking the authorization request to its callback.
// Should be URL-safe, Here alphanumeric characters.
func Oauth2State() string {
    return RandomString(Oauth2StateLength, alphanumericAlphabet)
}
func Oauth2CodeVerifier() string {
    return RandomString(OauthCodeVerifierLength, pkceAlphabet)
}

// S256Challenge creates base64 encoded sha256 challenge string derived from code.
// The padding of the result base64 string is stripped per [RFC 7636].
//
// [RFC 7636]: https://datatracker.ietf.org/doc/html/rfc7636#section-4.2
func S256Challenge(code string) string {
	h := sha256.New()
	h.Write([]byte(code))
	return strings.TrimRight(base64.URLEncoding.EncodeToString(h.Sum(nil)), "=")
}



