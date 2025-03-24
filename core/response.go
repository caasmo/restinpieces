package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/caasmo/restinpieces/db"
)

type jsonResponse struct {
	status int
	body   []byte
}

// JsonResponseWithData is used for structured JSON responses with optional data
type JsonResponseWithData struct {
	Status  int         `json:"status"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

var apiJsonDefaultHeaders = map[string]string{

	"Content-Type":              "application/json; charset=utf-8",

    // Ensure the browser respects the declared content type strictly.
    // mitigate MIME-type sniffing attacks
    // browsers sometimes "sniff" or guess the content type of a resource based on its
    // actual content, rather than strictly adhering to the Content-Type header.
    // Attackers can exploit this by uploading malicious content.
	"X-Content-Type-Options":    "nosniff",

    // The response must not be stored in any cache, anywhere, under any circumstances
    // no-store alone is enough to prevent all caching
    // no-cache and must-revalidate is just assurance if something downstream misinterprets no-store.
	"Cache-Control":             "no-store, no-cache, must-revalidate",

    // Prevents the response from being embedded in an <iframe>, mitigating clickjacking attacks
    // Adds a layer of defense against obscure misuse
	"X-Frame-Options":           "DENY",


    // Controls cross-origin resource sharing (CORS)
    // be restrictive, most restrictive is not to have it, same domain as api endpoints
    // TODO configurable
	//"Access-Control-Allow-Origin": "*",


    // HSTS TODO configurable  based on server are we under TLS terminating proxy
	//"Strict-Transport-Security": "max-age=31536000",
}

// TODO
var htmlHeaders = map[string]string{

    // CSP governs browser behavior for resources loaded as part of rendering a document
    // Prevents cross-site scripting (XSS) attacks by controlling which resources can be loaded.
    // means: “By default, only load resources from this server’s origin, nothing external.”
    // Unnecessary for pure API servers since they don't serve HTML/JavaScript
    "Content-Security-Policy":  "default-src 'self'",

    // mitigate reflected XSS attacks: malicious scripts are injected into a
    // page via user input (e.g., query parameters, form data) and then
    // "reflected" back to the user in the server’s response.
    // 1: Enables the browser’s XSS filter
    // mode=block: Instructs the browser to block the entire page if an XSS attack is detected
    //
    // Modern browsers (post-2019 Chrome, Edge, etc.) ignore this header, favoring Content Security Policy (CSP)
    // this header is mostly a legacy tool
    // Optional for API servers, but no harm
	//"X-XSS-Protection":           "1; mode=block",
}

// ApplyHeaders sets all headers from a map to the response writer
func setHeaders(w http.ResponseWriter, headers map[string]string) {
    for key, value := range headers {
	    w.Header()[key] = []string{value}
    }
}

// Standard error codes 
const (
	CodeTokenGeneration                = "token_generation"
	CodeClaimsNotFound                 = "claims_not_found"
	CodeInvalidRequest                 = "invalid_input"
	CodeInvalidCredentials             = "invalid_credentials"
	CodePasswordMismatch               = "password_mismatch"
	CodeMissingFields                  = "missing_fields"
	CodePasswordComplexity             = "password_complexity"
	CodeEmailConflict                  = "email_conflict"
	CodeNotFound                       = "not_found"
	CodeConflict                       = "conflict"
	CodeRegistrationFailed             = "registration_failed"
	CodeTooManyRequests                = "too_many_requests"
	CodeServiceUnavailable             = "service_unavailable"
	CodeNoAuthHeader                   = "no_auth_header"
	CodeInvalidTokenFormat             = "invalid_token_format"
	CodeJwtInvalidSignMethod           = "invalid_sign_method"
	CodeJwtTokenExpired                = "token_expired"
	CodeAlreadyVerified                = "already_verified"
	CodeJwtInvalidToken                = "invalid_token"
	CodeJwtInvalidVerificationToken    = "invalid_verification_token"
	CodeInvalidOAuth2Provider          = "invalid_oauth2_provider"
	CodeOAuth2TokenExchangeFailed      = "oauth2_token_exchange_failed"
	CodeOAuth2UserInfoFailed           = "oauth2_user_info_failed"
	CodeOAuth2UserInfoProcessingFailed = "oauth2_user_info_processing_failed"
	CodeOAuth2DatabaseError            = "oauth2_database_error"
	CodeAuthDatabaseError              = "auth_database_error"
)

// ResponseBasicFormat is used  for short ok and error responses
const shortFormat = `{"status":%d,"code":"%s","message":"%s"}`

// precomputeResponse() will be executed during initialization (before main() runs),
// and the JSON body will be precomputed and stored in the response variables.
// the variables will contain the fully JSON as []byte already
// It avoids repeated JSON marshaling during request handling
// Any time we use writeJSONResponse(w, response) in the code, it
// simply writes the pre-computed bytes to the response writer
func precomputeResponse(status int, code, message string) jsonResponse {
	body := fmt.Sprintf(shortFormat, status, code, message)
	return jsonResponse{status: status, body: []byte(body)}
}

// Precomputed error and ok responses with status codes
var (
	//errors
	errorTokenGeneration                = precomputeResponse(http.StatusInternalServerError, CodeTokenGeneration, "Failed to generate authentication token")
	errorClaimsNotFound                 = precomputeResponse(http.StatusInternalServerError, CodeClaimsNotFound, "Failed to generate token: Claims not found")
	errorInvalidRequest                 = precomputeResponse(http.StatusBadRequest, CodeInvalidRequest, "The request contains invalid data")
	errorInvalidCredentials             = precomputeResponse(http.StatusUnauthorized, CodeInvalidCredentials, "Invalid credentials provided")
	errorPasswordMismatch               = precomputeResponse(http.StatusBadRequest, CodePasswordMismatch, "Password and confirmation do not match")
	errorMissingFields                  = precomputeResponse(http.StatusBadRequest, CodeMissingFields, "Required fields are missing")
	errorPasswordComplexity             = precomputeResponse(http.StatusBadRequest, CodePasswordComplexity, "Password must be at least 8 characters")
	errorEmailConflict                  = precomputeResponse(http.StatusConflict, CodeEmailConflict, "Email address is already registered")
	errorNotFound                       = precomputeResponse(http.StatusNotFound, CodeNotFound, "Requested resource not found")
	errorConflict                       = precomputeResponse(http.StatusConflict, CodeConflict, "Verification already requested")
	errorRegistrationFailed             = precomputeResponse(http.StatusBadRequest, CodeRegistrationFailed, "Registration failed due to invalid data")
	errorTooManyRequests                = precomputeResponse(http.StatusTooManyRequests, CodeTooManyRequests, "Too many requests, please try again later")
	errorServiceUnavailable             = precomputeResponse(http.StatusServiceUnavailable, CodeServiceUnavailable, "Service is temporarily unavailable")
	errorNoAuthHeader                   = precomputeResponse(http.StatusUnauthorized, CodeNoAuthHeader, "Authorization header is required")
	errorInvalidTokenFormat             = precomputeResponse(http.StatusUnauthorized, CodeInvalidTokenFormat, "Invalid authorization token format")
	errorJwtInvalidSignMethod           = precomputeResponse(http.StatusUnauthorized, CodeJwtInvalidSignMethod, "Invalid JWT signing method")
	errorJwtTokenExpired                = precomputeResponse(http.StatusUnauthorized, CodeJwtTokenExpired, "Authentication token has expired")
	errorJwtInvalidToken                = precomputeResponse(http.StatusUnauthorized, CodeJwtInvalidToken, "Invalid authentication token")
	errorJwtInvalidVerificationToken    = precomputeResponse(http.StatusUnauthorized, CodeJwtInvalidVerificationToken, "Invalid verification token")
	errorEmailVerificationFailed        = precomputeResponse(http.StatusInternalServerError, "email_verification_failed", "Email verification process failed")
	errorInvalidOAuth2Provider          = precomputeResponse(http.StatusBadRequest, CodeInvalidOAuth2Provider, "Invalid OAuth2 provider specified")
	errorOAuth2TokenExchangeFailed      = precomputeResponse(http.StatusBadRequest, CodeOAuth2TokenExchangeFailed, "Failed to exchange OAuth2 token")
	errorOAuth2UserInfoFailed           = precomputeResponse(http.StatusBadRequest, CodeOAuth2UserInfoFailed, "Failed to get user info from OAuth2 provider")
	errorOAuth2UserInfoProcessingFailed = precomputeResponse(http.StatusBadRequest, CodeOAuth2UserInfoProcessingFailed, "Failed to process user info from OAuth2 provider")
	errorOAuth2DatabaseError            = precomputeResponse(http.StatusInternalServerError, CodeOAuth2DatabaseError, "Database error during OAuth2 authentication")
	errorAuthDatabaseError              = precomputeResponse(http.StatusInternalServerError, CodeAuthDatabaseError, "Database error during authentication")

	// oks
	okAlreadyVerified = precomputeResponse(http.StatusAccepted, "already_verified", "Email already verified - no further action needed")
	okEmailVerified   = precomputeResponse(http.StatusOK, "email_verified", "Email verified successfully")
)

// For successful short responses
func writeJsonOk(w http.ResponseWriter, resp jsonResponse) {
	w.WriteHeader(resp.status)
    setHeaders(w, apiJsonDefaultHeaders)
	w.Write(resp.body)
}

// writeJsonWithData writes a structured JSON response with the provided data
func writeJsonWithData(w http.ResponseWriter, resp JsonResponseWithData) {
	w.WriteHeader(resp.Status)
	setHeaders(w, apiJsonDefaultHeaders)
	json.NewEncoder(w).Encode(resp)
}

// writeJsonError writes a precomputed JSON error response
func writeJsonError(w http.ResponseWriter, resp jsonResponse) {
	w.WriteHeader(resp.status)
    setHeaders(w, apiJsonDefaultHeaders)
	w.Write(resp.body)
}

// writeAuthTokenResponse writes a standardized authentication token response
// Used for both password and OAuth2 authentication
// TODO move, too specific 
func writeAuthTokenResponse(w http.ResponseWriter, token string, expiresIn int, user *db.User) {
    setHeaders(w, apiJsonDefaultHeaders)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token_type":   "Bearer",
		"access_token": token,
		"expires_in":   expiresIn,
		"record": map[string]interface{}{
			"id":       user.ID,
			"email":    user.Email,
			"name":     user.Name,
			"verified": user.Verified,
		},
	})
}
