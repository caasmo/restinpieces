package crypto

import (
	"crypto/rand"
	"encoding/hex"
)

// generateSecureToken creates a cryptographically secure random token
// TODO
func GenerateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
