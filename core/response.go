package core

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

// JsonWithData is used for structured JSON responses with optional data
type JsonWithData struct {
	Status  int         `json:"status"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewJsonWithData creates a new JsonWithData instance
func NewJsonWithData(status int, code, message string, data interface{}) *JsonWithData {
	return &JsonWithData{
		Status:  status,
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// Standard response codes 
const (
    // oks
	CodeOkAuthentication              = "ok_authentication" // Standard success code for auth
	CodeOkAlreadyVerified             = "ok_already_verified"
	CodeOkEmailVerified               = "ok_email_verified"
	CodeOkOAuth2ProvidersList         = "ok_oauth2_providers_list" // Success code for OAuth2 providers list
	CodeOkVerificationRequested       = "ok_verification_requested" // Success code for email verification request



    //errors
	CodeErrorTokenGeneration                = "err_token_generation"
	CodeErrorClaimsNotFound                 = "err_claims_not_found"
	CodeErrorInvalidRequest                 = "err_invalid_input"
	CodeErrorInvalidCredentials             = "err_invalid_credentials"
	CodeErrorPasswordMismatch               = "err_password_mismatch"
	CodeErrorMissingFields                  = "err_missing_fields"
	CodeErrorPasswordComplexity             = "err_password_complexity"
	CodeErrorEmailConflict                  = "err_email_conflict"
	CodeErrorNotFound                       = "err_not_found"
	CodeErrorConflict                       = "err_conflict"
	CodeErrorRegistrationFailed             = "err_registration_failed"
	CodeErrorTooManyRequests                = "err_too_many_requests"
	CodeErrorServiceUnavailable             = "err_service_unavailable"
	CodeErrorNoAuthHeader                   = "err_no_auth_header"
	CodeErrorInvalidTokenFormat             = "err_invalid_token_format"
	CodeErrorJwtInvalidSignMethod           = "err_invalid_sign_method"
	CodeErrorJwtTokenExpired                = "err_token_expired"
	CodeErrorAlreadyVerified                = "err_already_verified"
	CodeErrorJwtInvalidToken                = "err_invalid_token"
	CodeErrorJwtInvalidVerificationToken    = "err_invalid_verification_token"
	CodeErrorInvalidOAuth2Provider          = "err_invalid_oauth2_provider"
	CodeErrorOAuth2TokenExchangeFailed      = "err_oauth2_token_exchange_failed"
	CodeErrorOAuth2UserInfoFailed           = "err_oauth2_user_info_failed"
	CodeErrorOAuth2UserInfoProcessingFailed = "err_oauth2_user_info_processing_failed"
	CodeErrorOAuth2DatabaseError            = "err_oauth2_database_error"
	CodeErrorAuthDatabaseError              = "err_auth_database_error"
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
	body := fmt.Sprintf(shortFormat, status, code, message)// TODO type
	return jsonResponse{status: status, body: []byte(body)}
}

// Precomputed error and ok responses with status codes
var (
	//errors
	errorTokenGeneration                = precomputeResponse(http.StatusInternalServerError, CodeErrorTokenGeneration, "Failed to generate authentication token")
	errorClaimsNotFound                 = precomputeResponse(http.StatusInternalServerError, CodeErrorClaimsNotFound, "Failed to generate token: Claims not found")
	errorInvalidRequest                 = precomputeResponse(http.StatusBadRequest, CodeErrorInvalidRequest, "The request contains invalid data")
	errorInvalidCredentials             = precomputeResponse(http.StatusUnauthorized, CodeErrorInvalidCredentials, "Invalid credentials provided")
	errorPasswordMismatch               = precomputeResponse(http.StatusBadRequest, CodeErrorPasswordMismatch, "Password and confirmation do not match")
	errorMissingFields                  = precomputeResponse(http.StatusBadRequest, CodeErrorMissingFields, "Required fields are missing")
	errorPasswordComplexity             = precomputeResponse(http.StatusBadRequest, CodeErrorPasswordComplexity, "Password must be at least 8 characters")
	errorEmailConflict                  = precomputeResponse(http.StatusConflict, CodeErrorEmailConflict, "Email address is already registered")
	errorNotFound                       = precomputeResponse(http.StatusNotFound, CodeErrorNotFound, "Requested resource not found")
	errorConflict                       = precomputeResponse(http.StatusConflict, CodeErrorConflict, "Verification already requested")
	errorRegistrationFailed             = precomputeResponse(http.StatusBadRequest, CodeErrorRegistrationFailed, "Registration failed due to invalid data")
	errorTooManyRequests                = precomputeResponse(http.StatusTooManyRequests, CodeErrorTooManyRequests, "Too many requests, please try again later")
	errorServiceUnavailable             = precomputeResponse(http.StatusServiceUnavailable, CodeErrorServiceUnavailable, "Service is temporarily unavailable")
	errorNoAuthHeader                   = precomputeResponse(http.StatusUnauthorized, CodeErrorNoAuthHeader, "Authorization header is required")
	errorInvalidTokenFormat             = precomputeResponse(http.StatusUnauthorized, CodeErrorInvalidTokenFormat, "Invalid authorization token format")
	errorJwtInvalidSignMethod           = precomputeResponse(http.StatusUnauthorized, CodeErrorJwtInvalidSignMethod, "Invalid JWT signing method")
	errorJwtTokenExpired                = precomputeResponse(http.StatusUnauthorized, CodeErrorJwtTokenExpired, "Authentication token has expired")
	errorJwtInvalidToken                = precomputeResponse(http.StatusUnauthorized, CodeErrorJwtInvalidToken, "Invalid authentication token")
	errorJwtInvalidVerificationToken    = precomputeResponse(http.StatusUnauthorized, CodeErrorJwtInvalidVerificationToken, "Invalid verification token")
	errorEmailVerificationFailed        = precomputeResponse(http.StatusInternalServerError, "err_email_verification_failed", "Email verification process failed")
	errorInvalidOAuth2Provider          = precomputeResponse(http.StatusBadRequest, CodeErrorInvalidOAuth2Provider, "Invalid OAuth2 provider specified")
	errorOAuth2TokenExchangeFailed      = precomputeResponse(http.StatusBadRequest, CodeErrorOAuth2TokenExchangeFailed, "Failed to exchange OAuth2 token")
	errorOAuth2UserInfoFailed           = precomputeResponse(http.StatusBadRequest, CodeErrorOAuth2UserInfoFailed, "Failed to get user info from OAuth2 provider")
	errorOAuth2UserInfoProcessingFailed = precomputeResponse(http.StatusBadRequest, CodeErrorOAuth2UserInfoProcessingFailed, "Failed to process user info from OAuth2 provider")
	errorOAuth2DatabaseError            = precomputeResponse(http.StatusInternalServerError, CodeErrorOAuth2DatabaseError, "Database error during OAuth2 authentication")
	errorAuthDatabaseError              = precomputeResponse(http.StatusInternalServerError, CodeErrorAuthDatabaseError, "Database error during authentication")

	// oks
	okAlreadyVerified = precomputeResponse(http.StatusAccepted, CodeOkAlreadyVerified, "Email already verified - no further action needed")
	okEmailVerified   = precomputeResponse(http.StatusOK, CodeOkEmailVerified, "Email verified successfully")
	okVerificationRequested = precomputeResponse(http.StatusAccepted, CodeOkVerificationRequested, "Verification email will be sent soon. Check your mailbox")
)

// For successful precomputed responses
func writeJsonOk(w http.ResponseWriter, resp jsonResponse) {
	w.WriteHeader(resp.status)
    setHeaders(w, apiJsonDefaultHeaders)
	w.Write(resp.body)
}

// writeJsonWithData writes a structured JSON response with the provided data
func writeJsonWithData(w http.ResponseWriter, resp JsonWithData) {
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

// AuthRecord represents the user record in authentication responses
type AuthRecord struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Verified bool   `json:"verified"`
}

// AuthData represents the authentication response structure
type AuthData struct {
	TokenType   string     `json:"token_type"`
	AccessToken string     `json:"access_token"`
	ExpiresIn   int        `json:"expires_in"`
	Record      AuthRecord `json:"record"`
}

// NewAuthData creates a new AuthData instance
func NewAuthData(token string, expiresIn int, user *db.User) *AuthData {
	return &AuthData{
		TokenType:   "Bearer",
		AccessToken: token,
		ExpiresIn:   expiresIn,
		Record: AuthRecord{
			ID:       user.ID,
			Email:    user.Email,
			Name:     user.Name,
			Verified: user.Verified,
		},
	}
}

// writeAuthResponse writes a standardized authentication response
func writeAuthResponse(w http.ResponseWriter, token string, expiresIn int, user *db.User) {
	authData := NewAuthData(token, expiresIn, user)
	response := NewJsonWithData(
		http.StatusOK,
		CodeOkAuthentication,
		"Authentication successful",
		authData,
	)
	writeJsonWithData(w, *response)
}
