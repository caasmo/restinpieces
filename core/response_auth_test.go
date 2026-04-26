package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

func TestWriteAuthResponse(t *testing.T) {
	w := httptest.NewRecorder()
	user := &db.User{
		ID:       "user123",
		Email:    "user@example.com",
		Name:     "John Doe",
		Verified: true,
	}
	token := "test-token"

	writeAuthResponse(w, token, user)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	expectedBody := `{"status":200,"code":"ok_authentication","message":"Authentication successful","data":{"token_type":"Bearer","access_token":"test-token","record":{"id":"user123","email":"user@example.com","name":"John Doe","verified":true}}}`
	actualBody := strings.TrimSpace(w.Body.String())
	if actualBody != expectedBody {
		t.Errorf("response body mismatch: got: %s want: %s", actualBody, expectedBody)
	}
}

func TestWriteOtpResponse(t *testing.T) {
	w := httptest.NewRecorder()
	token := "verify-token"

	writeOtpResponse(w, token)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	expectedBody := `{"status":200,"code":"ok_otp_token_issued","message":"Verification code sent","data":{"verification_token":"verify-token"}}`
	actualBody := strings.TrimSpace(w.Body.String())
	if actualBody != expectedBody {
		t.Errorf("response body mismatch: got: %s want: %s", actualBody, expectedBody)
	}
}

func TestWritePasswordResetOtpVerifiedResponse(t *testing.T) {
	w := httptest.NewRecorder()
	token := "reset-token"

	writePasswordResetOtpVerifiedResponse(w, token)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	expectedBody := `{"status":200,"code":"ok_password_reset_otp_verified","message":"OTP verified successfully","data":{"token":"reset-token"}}`
	actualBody := strings.TrimSpace(w.Body.String())
	if actualBody != expectedBody {
		t.Errorf("response body mismatch: got: %s want: %s", actualBody, expectedBody)
	}
}
