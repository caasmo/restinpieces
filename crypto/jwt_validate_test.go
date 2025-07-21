package crypto

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateClaimEmail(t *testing.T) {
	testCases := []struct {
		name      string
		claims    jwt.MapClaims
		wantError error
	}{
		{
			name:      "valid email",
			claims:    jwt.MapClaims{ClaimEmail: "test@example.com"},
			wantError: nil,
		},
		{
			name:      "missing email",
			claims:    jwt.MapClaims{},
			wantError: ErrClaimNotFound,
		},
		{
			name:      "empty email",
			claims:    jwt.MapClaims{ClaimEmail: ""},
			wantError: ErrInvalidClaimFormat,
		},
		{
			name:      "invalid email type",
			claims:    jwt.MapClaims{ClaimEmail: 123},
			wantError: ErrInvalidClaimFormat,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateClaimEmail(tc.claims[ClaimEmail])
			if !errors.Is(err, tc.wantError) {
				t.Errorf("validateClaimEmail() error = %v, want %v", err, tc.wantError)
			}
		})
	}
}

func TestValidateClaimType(t *testing.T) {
	testCases := []struct {
		name          string
		claims        jwt.MapClaims
		expectedValue string
		wantError     error
	}{
		{
			name:          "valid type",
			claims:        jwt.MapClaims{ClaimType: "verification"},
			expectedValue: "verification",
			wantError:     nil,
		},
		{
			name:          "missing type",
			claims:        jwt.MapClaims{},
			expectedValue: "verification",
			wantError:     ErrClaimNotFound,
		},
		{
			name:          "mismatched type",
			claims:        jwt.MapClaims{ClaimType: "password_reset"},
			expectedValue: "verification",
			wantError:     ErrInvalidClaimFormat,
		},
		{
			name:          "invalid type format",
			claims:        jwt.MapClaims{ClaimType: 123},
			expectedValue: "verification",
			wantError:     ErrInvalidClaimFormat,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateClaimType(tc.claims[ClaimType], tc.expectedValue)
			if !errors.Is(err, tc.wantError) {
				t.Errorf("validateClaimType() error = %v, want %v", err, tc.wantError)
			}
		})
	}
}

func TestValidateClaimExpiresAt(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		claims    jwt.MapClaims
		wantError error
	}{
		{
			name:      "valid exp",
			claims:    jwt.MapClaims{ClaimExpiresAt: float64(now.Add(1 * time.Minute).Unix())},
			wantError: nil,
		},
		{
			name:      "missing exp",
			claims:    jwt.MapClaims{},
			wantError: ErrClaimNotFound,
		},
		{
			name:      "expired token",
			claims:    jwt.MapClaims{ClaimExpiresAt: float64(now.Add(-1 * time.Minute).Unix())},
			wantError: ErrJwtTokenExpired,
		},
		{
			name:      "invalid exp type",
			claims:    jwt.MapClaims{ClaimExpiresAt: "not a number"},
			wantError: ErrInvalidClaimFormat,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateClaimExpiresAt(tc.claims[ClaimExpiresAt])
			if !errors.Is(err, tc.wantError) {
				t.Errorf("validateClaimExpiresAt() error = %v, want %v", err, tc.wantError)
			}
		})
	}
}

func TestValidateTypedClaims(t *testing.T) {
	now := time.Now()
	validClaims := jwt.MapClaims{
		ClaimIssuedAt:  float64(now.Unix()),
		ClaimExpiresAt: float64(now.Add(15 * time.Minute).Unix()),
		ClaimUserID:    "user123",
		ClaimEmail:     "test@example.com",
		ClaimNewEmail:  "new@example.com",
	}

	testCases := []struct {
		name          string
		claims        jwt.MapClaims
		validationFunc func(jwt.MapClaims) error
		wantError     error
	}{
		// Email Verification
		{
			name: "Valid Email Verification",
			claims: func() jwt.MapClaims {
				c := cloneClaims(validClaims)
				c[ClaimType] = ClaimVerificationValue
				return c
			}(),
			validationFunc: ValidateEmailVerificationClaims,
			wantError:     nil,
		},
		{
			name: "Invalid Email Verification - Missing iat",
			claims: func() jwt.MapClaims {
				c := cloneClaims(validClaims)
				c[ClaimType] = ClaimVerificationValue
				delete(c, ClaimIssuedAt)
				return c
			}(),
			validationFunc: ValidateEmailVerificationClaims,
			wantError:     ErrInvalidVerificationToken,
		},

		// Password Reset
		{
			name: "Valid Password Reset",
			claims: func() jwt.MapClaims {
				c := cloneClaims(validClaims)
				c[ClaimType] = ClaimPasswordResetValue
				return c
			}(),
			validationFunc: ValidatePasswordResetClaims,
			wantError:     nil,
		},

		// Session
		{
			name:           "Valid Session",
			claims:         validClaims,
			validationFunc: ValidateSessionClaims,
			wantError:      nil,
		},
		{
			name: "Invalid Session - Missing iat",
			claims: func() jwt.MapClaims {
				c := cloneClaims(validClaims)
				delete(c, ClaimIssuedAt)
				return c
			}(),
			validationFunc: ValidateSessionClaims,
			wantError:     ErrClaimNotFound,
		},

		// Email Change
		{
			name: "Valid Email Change",
			claims: func() jwt.MapClaims {
				c := cloneClaims(validClaims)
				c[ClaimType] = ClaimEmailChangeValue
				return c
			}(),
			validationFunc: ValidateEmailChangeClaims,
			wantError:     nil,
		},
		{
			name: "Invalid Email Change - Missing new_email",
			claims: func() jwt.MapClaims {
				c := cloneClaims(validClaims)
				c[ClaimType] = ClaimEmailChangeValue
				delete(c, ClaimNewEmail)
				return c
			}(),
			validationFunc: ValidateEmailChangeClaims,
			wantError:     ErrInvalidVerificationToken,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.validationFunc(tc.claims)
			if !errors.Is(err, tc.wantError) {
				t.Errorf("validation error = %v, want %v", err, tc.wantError)
			}
		})
	}
}

func cloneClaims(c jwt.MapClaims) jwt.MapClaims {
	clone := make(jwt.MapClaims, len(c))
	for k, v := range c {
		clone[k] = v
	}
	return clone
}
