package core

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	oauth2provider "github.com/caasmo/restinpieces/oauth2"
	"golang.org/x/oauth2"
)

// oauth2TokenExchangeTimeout defines the maximum duration for the OAuth2 token
// exchange step. Kept intentionally short — a legitimate provider responds in
// well under a second; 10 s is already generous.
const oauth2TokenExchangeTimeout = 10 * time.Second

// oauth2UserInfoTimeout is a separate, independent deadline for the user-info
// request that follows the token exchange.
//
// SECURITY: Sharing a single context between both network calls means a slow
// token exchange silently steals time from the user-info request, causing
// spurious failures that are indistinguishable from real errors. Each I/O
// leg gets its own deadline so errors are accurate and timeouts are fair.
const oauth2UserInfoTimeout = 5 * time.Second

// OAuth2ProviderInfo contains the provider details needed for client-side OAuth2 flow
type OAuth2ProviderInfo struct {
	Name                string `json:"name"`
	DisplayName         string `json:"displayName"`
	State               string `json:"state"`
	AuthURL             string `json:"authURL"`
	RedirectURL         string `json:"redirectURL"`
	CodeVerifier        string `json:"codeVerifier,omitempty"`
	CodeChallenge       string `json:"codeChallenge,omitempty"`
	CodeChallengeMethod string `json:"codeChallengeMethod,omitempty"`
}

// OAuth2ProviderListData wraps the list of providers for standardized response
type OAuth2ProviderListData struct {
	Providers []OAuth2ProviderInfo `json:"providers"`
}

type oauth2Request struct {
	Provider     string `json:"provider"`
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	State        string `json:"state"`
	RedirectURI  string `json:"redirect_uri"`
}

// AuthWithOAuth2Handler handles OAuth2 authentication and first-time registration.
// Endpoint: POST /auth-with-oauth2
// Authenticated: No
// Allowed Mimetype: application/json
//
// # Login vs Registration
//
// OAuth2 collapses the login/register distinction because the provider is the
// source of truth: if the email is unknown we create the account on the spot.
// This is the only write this handler performs.
//
// # Account method separation
//
// If the email already exists and was registered with a password, this handler
// rejects the request. Implicit account linking inside a login endpoint is a
// security risk — the user has no awareness it happened. Explicit linking is
// handled by the authenticated POST /link-oauth2 endpoint instead.
//
// # Race condition
//
// A TOCTOU window exists between GetUserByEmail returning nil and
// CreateUserWithOauth2 inserting the row. Two simultaneous first-time signups
// for the same address will race. The loser receives ErrConstraintUnique from
// the DB and gets a generic error response — acceptable for this edge case.
func (a *App) AuthWithOAuth2Handler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	var req oauth2Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if req.Provider == "" || req.Code == "" || req.CodeVerifier == "" || req.State == "" || req.RedirectURI == "" {
		WriteJsonError(w, errorMissingFields)
		return
	}

	// SECURITY (PKCE — RFC 7636 §4.1): Validate the code_verifier format before
	// using it. An empty or structurally invalid verifier would silently pass the
	// check above and reach the provider, which may reject or mishandle it in
	// provider-specific, hard-to-diagnose ways. Enforcing the spec here keeps
	// error surfaces narrow and predictable.
	if err := crypto.ValidateCodeVerifier(req.CodeVerifier); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Verify the JWT state is valid and cryptographically bound to the code_verifier.
	// This completely blocks Confused Deputy attacks (sending fake codes without a valid
	// state signature) and Login CSRF (intercepted codes missing the client's LocalStorage verifier).
	cfg := a.Config()
	if err := crypto.VerifyOauth2StateToken(req.State, req.CodeVerifier, cfg.Jwt.Oauth2StateSecret); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	provider, ok := cfg.OAuth2Providers[req.Provider]
	if !ok {
		WriteJsonError(w, errorInvalidOAuth2Provider)
		return
	}

	// SECURITY (open-redirect / code interception): The redirect URI must be
	// derived exclusively from server-side configuration. Accepting the value
	// supplied by the client allows an attacker who can craft a direct POST to
	// this endpoint (bypassing the JS CSRF check) to substitute their own URI
	// and potentially intercept authorization codes. The client-supplied value
	// is intentionally ignored here; redirectUrl() is the only source of truth.
	serverRedirectURI := redirectUrl(cfg.Server, provider)

	oauth2Config := oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  serverRedirectURI,
		Scopes:       provider.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  provider.AuthURL,
			TokenURL: provider.TokenURL,
		},
	}

	// Token exchange — dedicated context so its deadline is independent of the
	// user-info request that follows (see oauth2UserInfoTimeout above).
	exchangeCtx, exchangeCancel := context.WithTimeout(r.Context(), oauth2TokenExchangeTimeout)
	defer exchangeCancel()

	token, err := oauth2Config.Exchange(
		exchangeCtx,
		req.Code,
		oauth2.SetAuthURLParam("code_verifier", req.CodeVerifier),
	)
	if err != nil {
		WriteJsonError(w, errorOAuth2TokenExchangeFailed)
		return
	}

	// User-info fetch — fresh context with its own independent deadline.
	infoCtx, infoCancel := context.WithTimeout(r.Context(), oauth2UserInfoTimeout)
	defer infoCancel()

	client := oauth2Config.Client(infoCtx, token)
	resp, err := client.Get(provider.UserInfoURL)
	if err != nil {
		WriteJsonError(w, errorOAuth2UserInfoFailed)
		return
	}

    // http.Client.Get() returns a response whose body is an open network
    // stream. Even if you've finished reading it, the underlying TCP
    // connection stays open and occupied until the body is explicitly closed
    //
    // Go's http.Transport can reuse TCP connections (keep-alive), but only if
    // the body is fully drained and closed.
    defer func() { _ = resp.Body.Close() }()

	oauthUser, err := oauth2provider.UserFromUserInfoURL(resp, provider.Name)
	if err != nil {
		WriteJsonError(w, errorOAuth2UserInfoProcessingFailed)
		return
	}

	if oauthUser.Email == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// SECURITY (account deduplication): Normalize the email to lowercase before
	// any comparison or storage. Providers are inconsistent — some return
	// "User@Example.com", others "user@example.com". Without normalization, the
	// same real-world address can create multiple distinct accounts, breaking
	// the de-duplication logic that relies on email uniqueness.
	oauthUser.Email = strings.ToLower(strings.TrimSpace(oauthUser.Email))

	if err := a.Validator().Email(oauthUser.Email); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	user, err := a.DbAuth().GetUserByEmail(oauthUser.Email)
	if err != nil {
		WriteJsonError(w, errorOAuth2DatabaseError)
		return
	}

	if user == nil {
		// Unknown email — first-time OAuth2 signup. Create the account.
		user, err = a.DbAuth().CreateUserWithOauth2(*oauthUser)
		if err != nil {
			// ErrConstraintUnique here means a simultaneous signup for the
			// same address won the race. Surface as a generic error; the
			// client can retry and will hit the login path on the next attempt.
			WriteJsonError(w, errorOAuth2DatabaseError)
			return
		}
	} else if !user.Oauth2 {
		// Email exists but was registered with a password. Reject — implicit
		// account linking inside a login endpoint is not allowed. The user
		// must authenticate first and then explicitly link via POST /link-oauth2.
		WriteJsonError(w, errorEmailConflict)
		return
	}

	// Generate JWT session token.
	// Password is empty for pure OAuth2 accounts; that is intentional and fine.
	// Users who have both a password and OAuth2 (via /link-password) will have
	// it included in the signing key.
	jwtToken, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, cfg.Jwt.AuthSecret, cfg.Jwt.AuthTokenDuration.Duration)
	if err != nil {
		WriteJsonError(w, errorTokenGeneration)
		return
	}

	writeAuthResponse(w, jwtToken, user)
}

// redirectUrl returns the complete redirect URL to use for this provider.
// If RedirectURLPath is set, it combines with the server's base URL.
// Otherwise falls back to RedirectURL if set.
// Returns empty string if neither is configured.
func redirectUrl(srvConf config.Server, provider config.OAuth2Provider) string {
	if provider.RedirectURLPath != "" {
		return srvConf.BaseURL() + provider.RedirectURLPath
	}
	return provider.RedirectURL
}
