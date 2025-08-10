package crypto

import (
	"fmt"
	"time"
)

// IMPORTANT: Regarding Claim Presence and Validation
//
// The `jwt.Parse` function and its parser options (e.g., `jwt.WithExpirationRequired`)
// validate the values of standard claims like `exp` IF THEY ARE PRESENT.
// However, most of these options DO NOT enforce the PRESENCE of the claims.
// For example, without `WithExpirationRequired`, a token missing the `exp` claim is not rejected.
//
// Therefore, it is the responsibility of our custom `Valid()` methods below
// to explicitly check for the presence of all required claims, both standard
// (e.g., `c.IssuedAt == nil`) and custom (e.g., `c.UserID == ""`).
//
// The parser validates the token format, signature, and any standard claims required
// by its options, then it calls the appropriate `Valid()` method to perform all our
// application-specific validation rules.

// Valid implements the jwt.Claims interface.
// The standard claims (e.g. exp) are validated by the parser before this method is called.
// This method validates custom application-specific rules.
func (c SessionClaims) Valid() error {
	// Enforce 'iat' presence, as the parser does not do this by default.
	if c.IssuedAt == nil {
		return fmt.Errorf("%w: missing iat claim", ErrInvalidClaimFormat)
	}

	// Enforce application's max token age.
	if time.Since(c.IssuedAt.Time) > MaxTokenAge {
		return ErrTokenTooOld
	}

	// Enforce presence of our custom 'user_id' claim.
	if c.UserID == "" {
		return fmt.Errorf("%w: missing user_id", ErrInvalidClaimFormat)
	}

	return nil
}

// Valid implements the jwt.Claims interface for password reset tokens.
func (c PasswordResetClaims) Valid() error {
	if c.IssuedAt == nil {
		return fmt.Errorf("%w: missing iat claim", ErrInvalidVerificationToken)
	}
	if time.Since(c.IssuedAt.Time) > MaxTokenAge {
		return ErrTokenTooOld
	}
	if c.UserID == "" {
		return fmt.Errorf("%w: missing user_id", ErrInvalidVerificationToken)
	}
	if c.Email == "" {
		return fmt.Errorf("%w: missing email", ErrInvalidVerificationToken)
	}
	if c.Type != ClaimPasswordResetValue {
		return fmt.Errorf("%w: invalid type claim '%s'", ErrInvalidVerificationToken, c.Type)
	}
	return nil
}

// Valid implements the jwt.Claims interface for email verification tokens.
func (c EmailVerificationClaims) Valid() error {
	if c.IssuedAt == nil {
		return fmt.Errorf("%w: missing iat claim", ErrInvalidVerificationToken)
	}
	if time.Since(c.IssuedAt.Time) > MaxTokenAge {
		return ErrTokenTooOld
	}
	if c.UserID == "" {
		return fmt.Errorf("%w: missing user_id", ErrInvalidVerificationToken)
	}
	if c.Email == "" {
		return fmt.Errorf("%w: missing email", ErrInvalidVerificationToken)
	}
	if c.Type != ClaimVerificationValue {
		return fmt.Errorf("%w: invalid type claim '%s'", ErrInvalidVerificationToken, c.Type)
	}
	return nil
}

// Valid implements the jwt.Claims interface for email change tokens.
func (c EmailChangeClaims) Valid() error {
	if c.IssuedAt == nil {
		return fmt.Errorf("%w: missing iat claim", ErrInvalidVerificationToken)
	}
	if time.Since(c.IssuedAt.Time) > MaxTokenAge {
		return ErrTokenTooOld
	}
	if c.UserID == "" {
		return fmt.Errorf("%w: missing user_id", ErrInvalidVerificationToken)
	}
	if c.Email == "" {
		return fmt.Errorf("%w: missing email", ErrInvalidVerificationToken)
	}
	if c.NewEmail == "" {
		return fmt.Errorf("%w: missing new_email", ErrInvalidVerificationToken)
	}
	if c.Type != ClaimEmailChangeValue {
		return fmt.Errorf("%w: invalid type claim '%s'", ErrInvalidVerificationToken, c.Type)
	}
	return nil
}