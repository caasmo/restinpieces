package crypto

import (
)

// Defined in RFC 7636 (PKCE). Allowed characters: A-Z, a-z, 0-9, and the symbols -, ., _, ~.
const pkceAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"


// The OAuth2 specification (RFC 6749) doesn’t mandate a specific length. It
// recommends a random, unguessable string.
// At least 16 characters, though 32 to 64 characters is common
// for better uniqueness and security.
const Oauth2StateLength = 32

// Defined in RFC 7636 (PKCE). Its length must be between 43 and 128 characters.
const OauthCodeVerifierLength = 43

// The state parameter helps prevent Cross-Site Request Forgery (CSRF) attacks
// by linking the authorization request to its callback.
// Shoudl be URL-safe, Here alphanumeric characters.
func Oauth2State() string {
    return RandomString(Oauth2StateLength, alphanumericAlphabet)
}
func Oauth2CodeVerifier() string {
    return RandomString(OauthCodeVerifierLength, pkceAlphabet)
}



