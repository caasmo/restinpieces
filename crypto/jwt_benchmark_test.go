package crypto

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// setupCryptoBenchmark provides a consistent set of data for crypto benchmarks.
func setupCryptoBenchmark(b *testing.B) (userID, email, passwordHash, secret string, signingKey []byte) {
	b.Helper()

	userID = "user-test-123"
	email = "test@example.com"
	passwordHash = "$2a$10$VGE8iAnq4vS7g0/0cT/G.u2J5.C.6A3sJ/A6B.zY9C.X7D.E5F.G"
	secret = "a_super_secret_key_that_is_at_least_32_bytes_long"

	var err error
	signingKey, err = NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		b.Fatalf("Failed to create signing key: %v", err)
	}

	return
}

// --- Benchmark Scenarios ---

func BenchmarkJwt_NewSigningKey(b *testing.B) {
	_, email, passwordHash, secret, _ := setupCryptoBenchmark(b)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, _ = NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	}
}

func BenchmarkJwt_New(b *testing.B) {
	userID, _, _, _, signingKey := setupCryptoBenchmark(b)
	claims := jwt.MapClaims{ClaimUserID: userID}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, _ = NewJwt(claims, signingKey, time.Hour)
	}
}

func BenchmarkJwt_Parse_Valid(b *testing.B) {
	userID, _, _, _, signingKey := setupCryptoBenchmark(b)
	claims := jwt.MapClaims{ClaimUserID: userID}
	token, err := NewJwt(claims, signingKey, time.Hour)
	if err != nil {
		b.Fatalf("Failed to generate token for benchmark: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, _ = ParseJwt(token, signingKey)
	}
}

func BenchmarkJwt_Parse_InvalidSignature(b *testing.B) {
	userID, _, _, _, signingKey := setupCryptoBenchmark(b)
	claims := jwt.MapClaims{ClaimUserID: userID}
	token, err := NewJwt(claims, signingKey, time.Hour)
	if err != nil {
		b.Fatalf("Failed to generate token for benchmark: %v", err)
	}

	invalidKey := []byte("a_different_invalid_secret_key_32_bytes_long")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, _ = ParseJwt(token, invalidKey)
	}
}

func BenchmarkJwt_Parse_Unverified(b *testing.B) {
	userID, _, _, _, signingKey := setupCryptoBenchmark(b)
	claims := jwt.MapClaims{ClaimUserID: userID}
	token, err := NewJwt(claims, signingKey, time.Hour)
	if err != nil {
		b.Fatalf("Failed to generate token for benchmark: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, _ = ParseJwtUnverified(token)
	}
}
