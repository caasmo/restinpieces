package core

import (
	"encoding/json"
	"net/http"
)

// Standard response codes
const (
	// oks

	CodeOkAlreadyVerified       = "ok_already_verified"
	CodeOkEmailVerified         = "ok_email_verified"
	CodeOkVerificationRequested = "ok_verification_requested" // Success code for email verification request
	CodeOkPasswordResetRequested = "ok_password_reset_requested" // Success code for password reset request
	CodeOkEmailChangeRequested   = "ok_email_change_requested"   // Success code for email change request

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
	CodeErrorEmailVerificationAlreadyRequested = "err_email_verification_already_requested"
	CodeErrorPasswordResetAlreadyRequested  = "err_password_reset_already_requested"
	CodeErrorPasswordResetFailed            = "err_password_reset_failed"
	CodeOkPasswordReset                     = "ok_password_reset"
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
	CodeErrorIpBlocked                      = "err_ip_blocked"
	CodeErrorInvalidContentType             = "err_invalid_content_type"
	// oks
)

// ResponseBasicFormat is used  for short ok and error responses
// precomputeBasicResponse() will be executed during initialization (before main() runs),
// and the JSON body will be precomputed and stored in the response variables.
// the variables will contain the fully JSON as []byte already
// It avoids repeated JSON marshaling during request handling
// Any time we use writeJSONResponse(w, response) in the code, it
// simply writes the pre-computed bytes to the response writer
func precomputeBasicResponse(status int, code, message string) jsonResponse {
	basic := JsonBasic{
		Status:  status,
		Code:    code,
		Message: message,
	}
	body, _ := json.Marshal(basic)
	return jsonResponse{status: status, body: body}
}

// precomputeWithDataResponse creates a precomputed response with data that includes
// both basic response fields and additional payload data
func precomputeWithDataResponse(status int, code, message string, data interface{}) jsonResponse {
	response := JsonWithData{
		JsonBasic: JsonBasic{
			Status:  status,
			Code:    code,
			Message: message,
		},
		Data: data,
	}
	body, _ := json.Marshal(response)
	return jsonResponse{status: status, body: body}
}

// Precomputed error and ok responses with status codes
var (
	//errors
	errorTokenGeneration                = precomputeBasicResponse(http.StatusInternalServerError, CodeErrorTokenGeneration, "Failed to generate authentication token")
	errorIpBlocked                      = precomputeBasicResponse(http.StatusTooManyRequests, CodeErrorIpBlocked, "IP address has been blocked due to excessive requests. Please try again later")
	errorClaimsNotFound                 = precomputeBasicResponse(http.StatusInternalServerError, CodeErrorClaimsNotFound, "Failed to generate token: Claims not found")
	errorInvalidRequest                 = precomputeBasicResponse(http.StatusBadRequest, CodeErrorInvalidRequest, "The request contains invalid data")
	errorInvalidCredentials             = precomputeBasicResponse(http.StatusUnauthorized, CodeErrorInvalidCredentials, "Invalid credentials provided")
	errorPasswordMismatch               = precomputeBasicResponse(http.StatusBadRequest, CodeErrorPasswordMismatch, "Password and confirmation do not match")
	errorMissingFields                  = precomputeBasicResponse(http.StatusBadRequest, CodeErrorMissingFields, "Required fields are missing")
	errorPasswordComplexity             = precomputeBasicResponse(http.StatusBadRequest, CodeErrorPasswordComplexity, "Password must be at least 8 characters")
	errorEmailConflict                  = precomputeBasicResponse(http.StatusConflict, CodeErrorEmailConflict, "Email address is already registered")
	errorNotFound                       = precomputeBasicResponse(http.StatusNotFound, CodeErrorNotFound, "Requested resource not found")
	errorEmailVerificationAlreadyRequested = precomputeBasicResponse(http.StatusConflict, CodeErrorEmailVerificationAlreadyRequested, "Email verification already requested.")
	errorPasswordResetAlreadyRequested  = precomputeBasicResponse(http.StatusConflict, CodeErrorPasswordResetAlreadyRequested, "Password reset already requested")
	errorPasswordResetFailed            = precomputeBasicResponse(http.StatusInternalServerError, CodeErrorPasswordResetFailed, "Password reset process failed")
	okPasswordReset                     = precomputeBasicResponse(http.StatusOK, CodeOkPasswordReset, "Password reset successfully")
	errorRegistrationFailed             = precomputeBasicResponse(http.StatusBadRequest, CodeErrorRegistrationFailed, "Registration failed due to invalid data")
	errorTooManyRequests                = precomputeBasicResponse(http.StatusTooManyRequests, CodeErrorTooManyRequests, "Too many requests, please try again later")
	errorServiceUnavailable             = precomputeBasicResponse(http.StatusServiceUnavailable, CodeErrorServiceUnavailable, "Service is temporarily unavailable")
	errorNoAuthHeader                   = precomputeBasicResponse(http.StatusUnauthorized, CodeErrorNoAuthHeader, "Authorization header is required")
	errorInvalidTokenFormat             = precomputeBasicResponse(http.StatusUnauthorized, CodeErrorInvalidTokenFormat, "Invalid authorization token format")
	errorJwtInvalidSignMethod           = precomputeBasicResponse(http.StatusUnauthorized, CodeErrorJwtInvalidSignMethod, "Invalid JWT signing method")
	errorJwtTokenExpired                = precomputeBasicResponse(http.StatusUnauthorized, CodeErrorJwtTokenExpired, "Authentication token has expired")
	errorJwtInvalidToken                = precomputeBasicResponse(http.StatusUnauthorized, CodeErrorJwtInvalidToken, "Invalid authentication token")
	errorJwtInvalidVerificationToken    = precomputeBasicResponse(http.StatusUnauthorized, CodeErrorJwtInvalidVerificationToken, "Invalid verification token")
	errorEmailVerificationFailed        = precomputeBasicResponse(http.StatusInternalServerError, "err_email_verification_failed", "Email verification process failed")
	errorInvalidOAuth2Provider          = precomputeBasicResponse(http.StatusBadRequest, CodeErrorInvalidOAuth2Provider, "Invalid OAuth2 provider specified")
	errorOAuth2TokenExchangeFailed      = precomputeBasicResponse(http.StatusBadRequest, CodeErrorOAuth2TokenExchangeFailed, "Failed to exchange OAuth2 token")
	errorOAuth2UserInfoFailed           = precomputeBasicResponse(http.StatusBadRequest, CodeErrorOAuth2UserInfoFailed, "Failed to get user info from OAuth2 provider")
	errorOAuth2UserInfoProcessingFailed = precomputeBasicResponse(http.StatusBadRequest, CodeErrorOAuth2UserInfoProcessingFailed, "Failed to process user info from OAuth2 provider")
	errorOAuth2DatabaseError            = precomputeBasicResponse(http.StatusInternalServerError, CodeErrorOAuth2DatabaseError, "Database error during OAuth2 authentication")
	errorAuthDatabaseError              = precomputeBasicResponse(http.StatusInternalServerError, CodeErrorAuthDatabaseError, "Database error during authentication")
	errorInvalidContentType             = precomputeBasicResponse(http.StatusUnsupportedMediaType, CodeErrorInvalidContentType, "Unsupported media type")

	// oks
	okAlreadyVerified       = precomputeBasicResponse(http.StatusAccepted, CodeOkAlreadyVerified, "Email already verified - no further action needed")
	okEmailVerified         = precomputeBasicResponse(http.StatusOK, CodeOkEmailVerified, "Email verified successfully")
	okVerificationRequested = precomputeBasicResponse(http.StatusAccepted, CodeOkVerificationRequested, "Verification email will be sent soon. Check your mailbox")
	okPasswordResetRequested = precomputeBasicResponse(http.StatusAccepted, CodeOkPasswordResetRequested, "Password reset instructions will be sent to your email if it exists in our system")
	okEmailChangeRequested   = precomputeBasicResponse(http.StatusAccepted, CodeOkEmailChangeRequested, "Email change instructions will be sent to your new email address")
)

// For successful precomputed responses
func writeJsonOk(w http.ResponseWriter, resp jsonResponse) {
	setHeaders(w, HeadersJson)
	w.WriteHeader(resp.status)
	w.Write(resp.body)
}

// writeJsonError writes a precomputed JSON error response
func writeJsonError(w http.ResponseWriter, resp jsonResponse) {
	setHeaders(w, HeadersJson)
	w.WriteHeader(resp.status)
	w.Write(resp.body)
}
