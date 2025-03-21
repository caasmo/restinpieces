package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/caasmo/restinpieces/db"
)

type jsonError struct {
	status    int
	body []byte
}

var jsonHeader = []string{"application/json; charset=utf-8"}

// Standard error codes and messages
const (
	CodeTokenGeneration      = "token_generation"
	CodeClaimsNotFound       = "claims_not_found"
	CodeInvalidRequest       = "invalid_input"
	CodeInvalidCredentials   = "invalid_credentials"
	CodePasswordMismatch     = "password_mismatch"
	CodeMissingFields        = "missing_fields"
	CodePasswordComplexity   = "password_complexity"
	CodeEmailConflict        = "email_conflict"
	CodeNotFound             = "not_found"
	CodeConflict             = "conflict"
	CodeRegistrationFailed   = "registration_failed"
	CodeTooManyRequests      = "too_many_requests"
	CodeServiceUnavailable   = "service_unavailable"
	CodeNoAuthHeader         = "no_auth_header"
	CodeInvalidTokenFormat   = "invalid_token_format"
	CodeJwtInvalidSignMethod = "invalid_sign_method"
	CodeJwtTokenExpired      = "token_expired"
	CodeAlreadyVerified      = "already_verified"
	CodeJwtInvalidToken      = "invalid_token"
)

// Precomputed error responses with status codes
var (
	errorTokenGeneration      = jsonError{http.StatusInternalServerError, []byte(`{"status":500,"code":"token_generation","message":"Failed to generate authentication token"}`)}
	errorClaimsNotFound       = jsonError{http.StatusInternalServerError, []byte(`{"status":500,"code":"claims_not_found","message":"Failed to generate token: Claims not found"}`)}
	errorInvalidRequest       = jsonError{http.StatusBadRequest, []byte(`{"status":400,"code":"invalid_input","message":"The request contains invalid data"}`)}
	errorInvalidCredentials   = jsonError{http.StatusUnauthorized, []byte(`{"status":401,"code":"invalid_credentials","message":"Invalid credentials provided"}`)}
	errorPasswordMismatch     = jsonError{http.StatusBadRequest, []byte(`{"status":400,"code":"password_mismatch","message":"Password and confirmation do not match"}`)}
	errorMissingFields        = jsonError{http.StatusBadRequest, []byte(`{"status":400,"code":"missing_fields","message":"Required fields are missing"}`)}
	errorPasswordComplexity   = jsonError{http.StatusBadRequest, []byte(`{"status":400,"code":"password_complexity","message":"Password must be at least 8 characters"}`)}
	errorEmailConflict        = jsonError{http.StatusConflict, []byte(`{"status":409,"code":"email_conflict","message":"Email address is already registered"}`)}
	errorNotFound             = jsonError{http.StatusNotFound, []byte(`{"status":404,"code":"not_found","message":"Requested resource not found"}`)}
	errorConflict             = jsonError{http.StatusConflict, []byte(`{"status":409,"code":"conflict","message":"Verification already requested"}`)}
	errorRegistrationFailed   = jsonError{http.StatusBadRequest, []byte(`{"status":400,"code":"registration_failed","message":"Registration failed due to invalid data"}`)}
	errorTooManyRequests      = jsonError{http.StatusTooManyRequests, []byte(`{"status":429,"code":"too_many_requests","message":"Too many requests, please try again later"}`)}
	errorServiceUnavailable   = jsonError{http.StatusServiceUnavailable, []byte(`{"status":503,"code":"service_unavailable","message":"Service is temporarily unavailable"}`)}
	errorNoAuthHeader         = jsonError{http.StatusUnauthorized, []byte(`{"status":401,"code":"no_auth_header","message":"Authorization header is required"}`)}
	errorInvalidTokenFormat   = jsonError{http.StatusUnauthorized, []byte(`{"status":401,"code":"invalid_token_format","message":"Invalid authorization token format"}`)}
	errorJwtInvalidSignMethod = jsonError{http.StatusUnauthorized, []byte(`{"status":401,"code":"invalid_sign_method","message":"Invalid JWT signing method"}`)}
	errorJwtTokenExpired      = jsonError{http.StatusUnauthorized, []byte(`{"status":401,"code":"token_expired","message":"Authentication token has expired"}`)}
	errorAlreadyVerified      = jsonError{http.StatusConflict, []byte(`{"status":409,"code":"already_verified","message":"Account is already verified"}`)}
	errorJwtInvalidToken      = jsonError{http.StatusUnauthorized, []byte(`{"status":401,"code":"invalid_token","message":"Invalid authentication token"}`)}
)

// writeJSONError writes a precomputed JSON error response
func writeJSONError(w http.ResponseWriter, err jsonError) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(err.code)
	w.Write(err.body)
}

// writeJSONErrorf writes a formatted JSON error response with custom message
func writeJSONErrorf(w http.ResponseWriter, code int, status string, format string, args ...interface{}) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  code,
		"code":    status,
		"message": fmt.Sprintf(format, args...),
	})
}

// writeAuthTokenResponse writes a standardized authentication token response
// Used for both password and OAuth2 authentication
func writeAuthTokenResponse(w http.ResponseWriter, token string, expiresIn int, user *db.User) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token_type": "Bearer",
		"access_token": token,
		"expires_in": expiresIn,
		"record": map[string]interface{}{
			"id":       user.ID,
			"email":    user.Email,
			"name":     user.Name,
			"verified": user.Verified,
		},
	})
}
