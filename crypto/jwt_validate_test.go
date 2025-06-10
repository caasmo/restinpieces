package crypto
import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateClaimUserID(t *testing.T) {
	testCases := []struct {
		name      string
		claims    jwt.MapClaims
		wantError error
	}{
		{
			name:      "valid user_id",
			claims:    jwt.MapClaims{ClaimUserID: "user123"},
			wantError: nil,
		},
		{
			name:      "missing user_id in empty claims",
			claims:    jwt.MapClaims{},
			wantError: ErrClaimNotFound,
		},
		{
			name:      "missing user_id in non-empty claims",
			claims:    jwt.MapClaims{"foo": "bar"},
			wantError: ErrClaimNotFound,
		},
		{
			name:      "user_id as number",
			claims:    jwt.MapClaims{ClaimUserID: 123},
			wantError: ErrInvalidClaimFormat,
		},
		{
			name:      "empty user_id string",
			claims:    jwt.MapClaims{ClaimUserID: ""},
			wantError: ErrInvalidClaimFormat,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateClaimUserID(tc.claims)
			if !errors.Is(err, tc.wantError) {
				t.Errorf("ValidateClaimUserID() error = %v, want %v", err, tc.wantError)
			}
		})
	}
}

func TestValidateClaimIssuedAt(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		claims    jwt.MapClaims
		wantError error
	}{
		{
			name:      "valid iat",
			claims:    jwt.MapClaims{ClaimIssuedAt: float64(now.Add(-1 * time.Minute).Unix())},
			wantError: nil,
		},
		{
			name:      "missing iat",
			claims:    jwt.MapClaims{},
			wantError: ErrClaimNotFound,
		},
		{
			name:      "iat in future",
			claims:    jwt.MapClaims{ClaimIssuedAt: float64(now.Add(1 * time.Minute).Unix())},
			wantError: ErrTokenUsedBeforeIssued,
		},
		{
			name:      "invalid iat type",
			claims:    jwt.MapClaims{ClaimIssuedAt: "not a number"},
			wantError: ErrInvalidClaimFormat,
		},
		{
			name:      "iat in non-empty claims",
			claims:    jwt.MapClaims{"foo": "bar", ClaimIssuedAt: float64(now.Add(-1 * time.Minute).Unix())},
			wantError: nil,
		},
		{
			name:      "token too old",
			claims:    jwt.MapClaims{ClaimIssuedAt: float64(now.Add(-8 * 24 * time.Hour).Unix())},
			wantError: ErrTokenTooOld,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateClaimIssuedAt(tc.claims)
			if !errors.Is(err, tc.wantError) {
				t.Errorf("ValidateClaimIssuedAt() error = %v, want %v", err, tc.wantError)
			}
		})
	}
}

