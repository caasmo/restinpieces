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

// TODO move to request
type oauth2Request struct {
	Provider     string `json:"provider"`
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	State        string `json:"state"` // Must now be explicitly received
	RedirectURI  string `json:"redirect_uri"`
}

// AuthWithOAuth2Handler handles OAuth2 authentication
// Endpoint: POST /auth-with-oauth2
// Authenticated: No
// Allowed Mimetype: application/json
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

	// SECURITY: Authorization codes and PKCE verifiers are short-lived
	// credentials. Logging them — even at debug level — risks leaking them
	// into log aggregators, crash dumps, or observability pipelines where
	// retention and access controls are typically weaker than the auth store.
	// Log presence/absence only; never the values themselves.
	//a.Logger().Debug("OAuth2 fields",
//		"provider", req.Provider,
//		"code_present", req.Code != "",
//		"code_verifier_present", req.CodeVerifier != "",
//		"redirect_uri", req.RedirectURI)

	// Validate required fields (now explicitly includes req.State)
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
		//a.Logger().Debug("PKCE code_verifier validation failed", "error", err)
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Verify the JWT state is valid and cryptographically bound to the code_verifier
	// This completely blocks Confused Deputy attacks (sending fake codes without a valid
	// state signature) and Login CSRF (intercepted codes missing the client's LocalStorage verifier).
	cfg := a.Config()
	if err := crypto.VerifyOauth2StateToken(req.State, req.CodeVerifier, cfg.Jwt.Oauth2StateSecret); err != nil {
		a.Logger().Warn("invalid oauth2 state token", "error", err)
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Get provider config
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

	a.Logger().Debug("Creating OAuth2 config", "provider", req.Provider, "scopes", provider.Scopes)
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
	a.Logger().Debug("Exchanging OAuth2 code for token", "provider", req.Provider)
	exchangeCtx, exchangeCancel := context.WithTimeout(r.Context(), oauth2TokenExchangeTimeout)
	defer exchangeCancel()

	token, err := oauth2Config.Exchange(
		exchangeCtx,
		req.Code,
		oauth2.SetAuthURLParam("code_verifier", req.CodeVerifier),
	)
	a.Logger().Debug("OAuth2 token exchange completed", "provider", req.Provider, "token", token != nil)
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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			a.Logger().Warn("failed to close response body", "error", err)
		}
	}()

	oauthUser, err := oauth2provider.UserFromUserInfoURL(resp, provider.Name)
	if err != nil {
		a.Logger().Debug("Failed to map provider user info", "error", err)
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

	// At this point we can say the user has registered from the point of the provider and the user.
	// - if the user exists and have ExternalAuth oauth2, we do not need to
	//   create: the user registered before with this or another auth provider
	// - if the user exists and have ExternalAuth "", the user has already register with another method, like password.
	// - if the user does not exists, we create record with auth
	//
	// Below we try to read from the Users table and then potentially write.
	// We coudl choose not to read but given that oauth2 distintion between
	// login and register is minimal, we want to minimize the number of writes.
	//
	// This could be a source of race condition and as result data inconsistency.
	//
	// we want though avoid transactions, and make a design that allow no
	// transactions while keeping data integrity.
	// the potential race condition could be two goroutines trying to write in
	// the same row(same email):
	// - user login/register with two different oauths providers at the same time
	// - user login/register one oauth2 provider, one with password
	// - user login/register with two different passwords, ?????TODO
	//
	// with sqlite being just "one writer at at time" we can avoid transactions by using simply a UNIQUE in the table.
	// If the two goroutines write, the last will lose an the goroutine will
	// provide a error: that is the same as we would have a transaction.
	//
	// But we go one step further:
	//
	// with the design of the methods CreateUserWithPassword and
	// CreateUserWithOauth2, we allow both competing goroutines to both always
	// succeed (on email conflict): ON CONFLICT updates the record by modifying fields that do not create
	// data inconsistency: In the case of two oauth2 registers, the first
	// will succeed and the second will just not write any new fields. the second user can
	// be informed that the user already exist by seeing the difference in its intended
	// avatar and the present (or other fields).
	// in the case of two conflicting auth methods, each one will write its
	// relevant fields (password, ExternalAuth), and the looser gorotuine can
	// also inform the user of existing user.
	user, err := a.DbAuth().GetUserByEmail(oauthUser.Email)
	if err != nil {
		WriteJsonError(w, errorOAuth2DatabaseError)
		return
	}

	// Create or update user with OAuth2 info if user doesn't exist or has false Oauth2
	if user == nil || !user.Oauth2 {
		user, err = a.DbAuth().CreateUserWithOauth2(*oauthUser)
		if err != nil {
			WriteJsonError(w, errorOAuth2DatabaseError)
			return
		}
	}

	// Generate JWT session token
	// If user has no password, because he logged in always with oauth2,
	// password is empty and thats fine. But the user can have both password and auth.
	// We always pass to the signingkey the passwordHash
	//a.Logger().Debug("Generating JWT for user", "userID", user.ID)
	jwtToken, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, cfg.Jwt.AuthSecret, cfg.Jwt.AuthTokenDuration.Duration)
	if err != nil {
		WriteJsonError(w, errorTokenGeneration)
		return
	}

	//a.Logger().Debug("Preparing successful authentication response")
	writeAuthResponse(w, jwtToken, user)
}

// RedirectURL returns the complete redirect URL to use for this provider.
// If RedirectURLPath is set, it combines with the server's base URL.
// Otherwise falls back to RedirectURL if set.
// Returns empty string if neither is configured.
func redirectUrl(srvConf config.Server, provider config.OAuth2Provider) string {
	if provider.RedirectURLPath != "" {
		return srvConf.BaseURL() + provider.RedirectURLPath
	}
	return provider.RedirectURL
}
