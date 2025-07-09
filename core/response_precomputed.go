package core

import (
	"encoding/json"
	"net/http"
)

// Standard response codes
const (
	// oks

	CodeOkAlreadyVerified        = "ok_already_verified"
	CodeOkEmailVerified          = "ok_email_verified"
	CodeOkVerificationRequested  = "ok_verification_requested"    // Success code for email verification request
	CodeOkPasswordResetRequested = "ok_password_reset_requested"  // Success code for password reset request
	CodeOkEmailChange            = "ok_email_change"              // Success code for completed email change
	CodeOkEmailChangeRequested   = "ok_email_change_requested"    // Success code for email change request
	CodeOkPasswordResetNotNeeded = "ok_password_reset_not_needed" // Success code when password reset is not needed
	CodeOkPasswordNotRequired    = "ok_password_not_required"     // Success code when password is not required for auth

	//errors
	CodeErrorTokenGeneration                   = "err_token_generation"
	CodeErrorClaimsNotFound                    = "err_claims_not_found"
	CodeErrorInvalidRequest                    = "err_invalid_input"
	CodeErrorInvalidCredentials                = "err_invalid_credentials"
	CodeErrorPasswordMismatch                  = "err_password_mismatch"
	CodeErrorMissingFields                     = "err_missing_fields"
	CodeErrorPasswordComplexity                = "err_password_complexity"
	CodeErrorEmailConflict                     = "err_email_conflict"
	CodeErrorNotFound                          = "err_not_found"
	CodeErrorEmailVerificationAlreadyRequested = "err_email_verification_already_requested"
	CodeErrorPasswordResetAlreadyRequested     = "err_password_reset_already_requested"
	CodeErrorEmailChangeAlreadyRequested       = "err_email_change_already_requested"
	CodeErrorPasswordResetFailed               = "err_password_reset_failed"
	CodeOkPasswordReset                        = "ok_password_reset"
	CodeErrorRegistrationFailed                = "err_registration_failed"
	CodeErrorTooManyRequests                   = "err_too_many_requests"
	CodeErrorServiceUnavailable                = "err_service_unavailable"
	CodeErrorNoAuthHeader                      = "err_no_auth_header"
	CodeErrorInvalidTokenFormat                = "err_invalid_token_format"
	CodeErrorJwtInvalidSignMethod              = "err_invalid_sign_method"
	CodeErrorJwtTokenExpired                   = "err_token_expired"
	CodeErrorAlreadyVerified                   = "err_already_verified"
	CodeErrorJwtInvalidToken                   = "err_invalid_token"
	CodeErrorJwtInvalidVerificationToken       = "err_invalid_verification_token"
	CodeErrorInvalidOAuth2Provider             = "err_invalid_oauth2_provider"
	CodeErrorOAuth2TokenExchangeFailed         = "err_oauth2_token_exchange_failed"
	CodeErrorOAuth2UserInfoFailed              = "err_oauth2_user_info_failed"
	CodeErrorOAuth2UserInfoProcessingFailed    = "err_oauth2_user_info_processing_failed"
	CodeErrorOAuth2DatabaseError               = "err_oauth2_database_error"
	CodeErrorAuthDatabaseError                 = "err_auth_database_error"
	CodeErrorIpBlocked                         = "err_ip_blocked"
	CodeErrorInvalidContentType                = "err_invalid_content_type"
	CodeErrorUnverifiedEmail                   = "err_unverified_email"
	// oks
)

// ResponseBasicFormat is used  for short ok and error responses
// PrecomputeBasicResponse() will be executed during initialization (before main() runs),
// and the JSON body will be precomputed and stored in the response variables.
// the variables will contain the fully JSON as []byte already
// It avoids repeated JSON marshaling during request handling
// Any time we use writeJSONResponse(w, response) in the code, it
// simply writes the pre-computed bytes to the response writer
func PrecomputeBasicResponse(status int, code, message string) jsonResponse {
	basic := JsonBasic{
		Status:  status,
		Code:    code,
		Message: message,
	}
	body, _ := json.Marshal(basic)
	return jsonResponse{status: status, body: body}
}

// Precomputed error and ok responses with status codes
var (

	//errors
	errorTokenGeneration                   = PrecomputeBasicResponse(http.StatusInternalServerError, CodeErrorTokenGeneration, "Failed to generate authentication token")
	errorInvalidRequest                    = PrecomputeBasicResponse(http.StatusBadRequest, CodeErrorInvalidRequest, "The request contains invalid data")
	errorInvalidCredentials                = PrecomputeBasicResponse(http.StatusUnauthorized, CodeErrorInvalidCredentials, "Invalid credentials provided")
	errorPasswordMismatch                  = PrecomputeBasicResponse(http.StatusBadRequest, CodeErrorPasswordMismatch, "Password and confirmation do not match")
	errorMissingFields                     = PrecomputeBasicResponse(http.StatusBadRequest, CodeErrorMissingFields, "Required fields are missing")
	errorPasswordComplexity                = PrecomputeBasicResponse(http.StatusBadRequest, CodeErrorPasswordComplexity, "Password must be at least 8 characters")
	errorEmailConflict                     = PrecomputeBasicResponse(http.StatusConflict, CodeErrorEmailConflict, "Email address is already registered")
	errorNotFound                          = PrecomputeBasicResponse(http.StatusNotFound, CodeErrorNotFound, "Requested resource not found")
	errorEmailVerificationAlreadyRequested = PrecomputeBasicResponse(http.StatusConflict, CodeErrorEmailVerificationAlreadyRequested, "Email verification already requested.")
	errorPasswordResetAlreadyRequested     = PrecomputeBasicResponse(http.StatusConflict, CodeErrorPasswordResetAlreadyRequested, "Password reset already requested")
	errorEmailChangeAlreadyRequested       = PrecomputeBasicResponse(http.StatusConflict, CodeErrorEmailChangeAlreadyRequested, "Email change already requested")
	errorPasswordResetFailed               = PrecomputeBasicResponse(http.StatusInternalServerError, CodeErrorPasswordResetFailed, "Password reset process failed")
	errorServiceUnavailable                = PrecomputeBasicResponse(http.StatusServiceUnavailable, CodeErrorServiceUnavailable, "Service is temporarily unavailable")
	errorNoAuthHeader                      = PrecomputeBasicResponse(http.StatusUnauthorized, CodeErrorNoAuthHeader, "Authorization header is required")
	errorInvalidTokenFormat                = PrecomputeBasicResponse(http.StatusUnauthorized, CodeErrorInvalidTokenFormat, "Invalid authorization token format")
	errorJwtInvalidSignMethod              = PrecomputeBasicResponse(http.StatusUnauthorized, CodeErrorJwtInvalidSignMethod, "Invalid JWT signing method")
	errorJwtTokenExpired                   = PrecomputeBasicResponse(http.StatusUnauthorized, CodeErrorJwtTokenExpired, "Authentication token has expired")
	errorJwtInvalidToken                   = PrecomputeBasicResponse(http.StatusUnauthorized, CodeErrorJwtInvalidToken, "Invalid authentication token")
	errorJwtInvalidVerificationToken       = PrecomputeBasicResponse(http.StatusUnauthorized, CodeErrorJwtInvalidVerificationToken, "Invalid verification token")
	errorEmailVerificationFailed           = PrecomputeBasicResponse(http.StatusInternalServerError, "err_email_verification_failed", "Email verification process failed")
	errorInvalidOAuth2Provider             = PrecomputeBasicResponse(http.StatusBadRequest, CodeErrorInvalidOAuth2Provider, "Invalid OAuth2 provider specified")
	errorOAuth2TokenExchangeFailed         = PrecomputeBasicResponse(http.StatusBadRequest, CodeErrorOAuth2TokenExchangeFailed, "Failed to exchange OAuth2 token")
	errorOAuth2UserInfoFailed              = PrecomputeBasicResponse(http.StatusBadRequest, CodeErrorOAuth2UserInfoFailed, "Failed to get user info from OAuth2 provider")
	errorOAuth2UserInfoProcessingFailed    = PrecomputeBasicResponse(http.StatusBadRequest, CodeErrorOAuth2UserInfoProcessingFailed, "Failed to process user info from OAuth2 provider")
	errorOAuth2DatabaseError               = PrecomputeBasicResponse(http.StatusInternalServerError, CodeErrorOAuth2DatabaseError, "Database error during OAuth2 authentication")
	errorAuthDatabaseError                 = PrecomputeBasicResponse(http.StatusInternalServerError, CodeErrorAuthDatabaseError, "Database error during authentication")
	errorInvalidContentType                = PrecomputeBasicResponse(http.StatusUnsupportedMediaType, CodeErrorInvalidContentType, "Unsupported media type")
	errorUnverifiedEmail                   = PrecomputeBasicResponse(http.StatusForbidden, CodeErrorUnverifiedEmail, "Email must be verified before changing it")

	// oks
	okPasswordReset          = PrecomputeBasicResponse(http.StatusOK, CodeOkPasswordReset, "Password reset successfully")
	okAlreadyVerified        = PrecomputeBasicResponse(http.StatusAccepted, CodeOkAlreadyVerified, "Email already verified - no further action needed")
	okEmailVerified          = PrecomputeBasicResponse(http.StatusOK, CodeOkEmailVerified, "Email verified successfully")
	okVerificationRequested  = PrecomputeBasicResponse(http.StatusAccepted, CodeOkVerificationRequested, "Verification email will be sent soon. Check your mailbox")
	okPasswordResetRequested = PrecomputeBasicResponse(http.StatusAccepted, CodeOkPasswordResetRequested, "Password reset instructions will be sent to your email if it exists in our system")
	okEmailChangeRequested   = PrecomputeBasicResponse(http.StatusAccepted, CodeOkEmailChangeRequested, "Email change instructions will be sent to your new email address")
	okEmailChange            = PrecomputeBasicResponse(http.StatusOK, CodeOkEmailChange, "Email change was completed")
	okPasswordResetNotNeeded = PrecomputeBasicResponse(http.StatusOK, CodeOkPasswordResetNotNeeded, "Password reset is not needed")
	okPasswordNotRequired    = PrecomputeBasicResponse(http.StatusOK, CodeOkPasswordNotRequired, "Current authentication does not require password")
)

// For successful precomputed responses
func WriteJsonOk(w http.ResponseWriter, resp jsonResponse) {
	SetHeaders(w, HeadersJson)
	w.WriteHeader(resp.status)
	_, _ = w.Write(resp.body)
}

// writeJsonError writes a precomputed JSON error response
func WriteJsonError(w http.ResponseWriter, resp jsonResponse) {
	SetHeaders(w, HeadersJson)
	w.WriteHeader(resp.status)
	_, _ = w.Write(resp.body)
}
