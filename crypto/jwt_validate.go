package crypto

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func ValidateEmailVerificationClaims(claims jwt.MapClaims) error {
	// Validate iat claim and token age
	if err := validateClaimIssuedAt(claims[ClaimIssuedAt]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate exp claim
	if err := validateClaimExpiresAt(claims[ClaimExpiresAt]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate user_id claim
	if err := validateClaimUserID(claims[ClaimUserID]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate required claims exist
	if err := validateClaimEmail(claims[ClaimEmail]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	if err := validateClaimType(claims[ClaimType], ClaimVerificationValue); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	return nil
}

func ValidatePasswordResetClaims(claims jwt.MapClaims) error {
	// Validate iat claim and token age
	if err := validateClaimIssuedAt(claims[ClaimIssuedAt]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate exp claim
	if err := validateClaimExpiresAt(claims[ClaimExpiresAt]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate user_id claim
	if err := validateClaimUserID(claims[ClaimUserID]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate required claims exist
	if err := validateClaimEmail(claims[ClaimEmail]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	if err := validateClaimType(claims[ClaimType], ClaimPasswordResetValue); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	return nil
}

func validateClaimIssuedAt(iat any) error {
	if iat == nil {
		return ErrClaimNotFound
	}

	if iatTime, ok := iat.(float64); ok {
		iatUnix := int64(iatTime)
		nowUnix := time.Now().Unix()
		if iatUnix > nowUnix {
			return ErrTokenUsedBeforeIssued
		}
		if nowUnix-iatUnix > MaxTokenAge {
			return ErrTokenTooOld
		}
		return nil
	}
	return ErrInvalidClaimFormat
}

func validateClaimEmail(email any) error {
	if email == nil {
		return ErrClaimNotFound
	}

	if emailStr, ok := email.(string); ok {
		if emailStr == "" {
			return ErrInvalidClaimFormat
		}
		return nil
	}
	return ErrInvalidClaimFormat
}

func validateClaimType(typeVal any, expectedValue string) error {
	if typeVal == nil {
		return ErrClaimNotFound
	}

	if typeStr, ok := typeVal.(string); ok {
		if typeStr != expectedValue {
			return ErrInvalidClaimFormat
		}
		return nil
	}
	return ErrInvalidClaimFormat
}

func validateClaimExpiresAt(exp any) error {
	if exp == nil {
		return ErrClaimNotFound
	}

	if expTime, ok := exp.(float64); ok {
		now := time.Now().Unix()
		if int64(expTime) < now {
			return ErrJwtTokenExpired
		}
		return nil
	}
	return ErrInvalidClaimFormat
}

func validateClaimUserID(userID any) error {
	if userID == nil {
		return ErrClaimNotFound
	}

	if userIDStr, ok := userID.(string); ok {
		if userIDStr == "" {
			return ErrInvalidClaimFormat
		}
		return nil
	}
	return ErrInvalidClaimFormat
}

func ValidateSessionClaims(claims jwt.MapClaims) error {
	// Validate iat claim and token age
	if err := validateClaimIssuedAt(claims[ClaimIssuedAt]); err != nil {
		return fmt.Errorf("%w: %v", ErrJwtInvalidToken, err)
	}

	// Validate exp claim - return raw expiration error
	if err := validateClaimExpiresAt(claims[ClaimExpiresAt]); err != nil {
		return err // Return unwrapped error
	}

	// Validate user_id claim
	if err := validateClaimUserID(claims[ClaimUserID]); err != nil {
		return fmt.Errorf("%w: %v", ErrJwtInvalidToken, err)
	}

	return nil
}

func ValidateEmailChangeClaims(claims jwt.MapClaims) error {
	// Validate iat claim and token age
	if err := validateClaimIssuedAt(claims[ClaimIssuedAt]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate exp claim
	if err := validateClaimExpiresAt(claims[ClaimExpiresAt]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate user_id claim
	if err := validateClaimUserID(claims[ClaimUserID]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate required claims exist
	if err := validateClaimEmail(claims[ClaimEmail]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate new email claim
	if err := validateClaimEmail(claims[ClaimNewEmail]); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	if err := validateClaimType(claims[ClaimType], ClaimEmailChangeValue); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	return nil
}
