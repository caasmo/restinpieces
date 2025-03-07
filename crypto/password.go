package crypto

import "golang.org/x/crypto/bcrypt"

// CheckPassword compares a bcrypt hashed password with its possible plaintext equivalent
func CheckPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

// GenerateHash creates a bcrypt hash from a password using reasonable default cost
func GenerateHash(password string) (string, error) {
    hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(hashedBytes), err
}
