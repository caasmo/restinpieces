package crypto

import (
	"fmt"
	"time"
)

// Valid checks the standard claims and the custom UserID claim.
func (c SessionClaims) Valid() error {
	if c.UserID == "" {
		return fmt.Errorf("%w: missing user_id", ErrInvalidClaimFormat)
	}
	// Check if the token is too old
	if c.IssuedAt != nil && time.Since(c.IssuedAt.Time) > MaxTokenAge {
		return ErrTokenTooOld
	}
	return nil
}

// Valid checks the standard claims and custom verification claims.
func (c VerificationClaims) Valid() error {
	if c.UserID == "" {
		return fmt.Errorf("%w: missing user_id", ErrInvalidVerificationToken)
	}
	if c.Email == "" {
		return fmt.Errorf("%w: missing email", ErrInvalidVerificationToken)
	}
	switch c.Type {
	case ClaimVerificationValue, ClaimPasswordResetValue:
		// Valid type
	default:
		return fmt.Errorf("%w: invalid type claim '%s'", ErrInvalidVerificationToken, c.Type)
	}
	// Check if the token is too old
	if c.IssuedAt != nil && time.Since(c.IssuedAt.Time) > MaxTokenAge {
		return ErrTokenTooOld
	}
	return nil
}

// Valid checks the standard claims and custom email change claims.
func (c EmailChangeClaims) Valid() error {
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
	// Check if the token is too old
	if c.IssuedAt != nil && time.Since(c.IssuedAt.Time) > MaxTokenAge {
		return ErrTokenTooOld
	}
	return nil
}