package config

import (
	"testing"
)

func TestEndpointsHash(t *testing.T) {
	e := Endpoints{
		RefreshAuth:                 "POST /api/refresh-auth",
		ListEndpoints:               "GET /api/list-endpoints",
		AuthWithPassword:            "POST /api/auth-with-password",
		RegisterWithPassword:        "POST /api/register-with-password",
		ListOAuth2Providers:         "GET /api/list-oauth2-providers",
		AuthWithOAuth2:              "POST /api/auth-with-oauth2",
		RequestPasswordResetOtp:     "POST /api/request-password-reset-otp",
		VerifyPasswordResetOtp:      "POST /api/verify-password-reset-otp",
		ConfirmPasswordResetOtp:     "POST /api/confirm-password-reset-otp",
		RequestEmailChangeOtp:       "POST /api/request-email-change-otp",
		ConfirmEmailChangeOtp:       "POST /api/confirm-email-change-otp",
		RequestEmailVerificationOtp: "POST /api/request-email-verification-otp",
		ConfirmEmailVerificationOtp: "POST /api/confirm-email-verification-otp",
	}

	hash1 := e.Hash()
	hash2 := e.Hash()

	if hash1 != hash2 {
		t.Errorf("Hash is not deterministic: got %q and %q", hash1, hash2)
	}

	if len(hash1) != 16 {
		t.Errorf("Expected hash length 16 (8 bytes hex), got %d: %q", len(hash1), hash1)
	}
}

func TestEndpointsHashChanges(t *testing.T) {
	e1 := Endpoints{
		AuthWithPassword: "POST /api/auth-with-password",
	}
	e2 := Endpoints{
		AuthWithPassword: "POST /api/v2/auth-with-password",
	}

	if e1.Hash() == e2.Hash() {
		t.Error("Different endpoints should produce different hashes")
	}
}
