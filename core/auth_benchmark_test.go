package core

import (
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
)

// setupAuthBenchmark initializes a consistent environment for the authenticator benchmarks.
// It uses the official mock database from `db/mock`.
func setupAuthBenchmark(b *testing.B) (*DefaultAuthenticator, *mock.Db, *config.Config, *db.User) {
	b.Helper()

	cfg := config.NewDefaultConfig()
	// Discard logger to prevent I/O from affecting benchmark results.
	logger := slog.New(slog.NewTextHandler(nil, nil))
	// Use the existing, official mock database.
	mockDB := &mock.Db{}
	provider := config.NewProvider(cfg)
	// The mock.Db struct implements the db.DbAuth interface, so this is valid.
	auth := NewDefaultAuthenticator(mockDB, logger, provider)

	// A standard user for generating tokens and for the mock DB to return.
	// The password hash is a realistic bcrypt hash.
	testUser := &db.User{
		ID:       "r2e4d72d378c747", // Use an ID that matches the format expected by parseJwtUserID.
		Email:    "test@example.com",
		Password: "$2a$10$VGE8iAnq4vS7g0/0cT/G.u2J5.C.6A3sJ/A6B.zY9C.X7D.E5F.G",
	}

	return auth, mockDB, cfg, testUser
}

// generateTestToken creates a JWT for benchmarking purposes using the project's crypto package.
func generateTestToken(b *testing.B, user *db.User, cfg *config.Config, duration time.Duration) string {
	b.Helper()
	token, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, cfg.Jwt.AuthSecret, duration)
	if err != nil {
		b.Fatalf("Failed to generate test token: %v", err)
	}
	return token
}

// --- Benchmark Scenarios ---

// BenchmarkAuthenticator_HappyPath measures the performance of a complete, successful authentication.
func BenchmarkAuthenticator_HappyPath(b *testing.B) {
	auth, mockDB, cfg, user := setupAuthBenchmark(b)
	token := generateTestToken(b, user, cfg, time.Hour)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Configure mock DB to return the valid user.
	mockDB.GetUserByIdFunc = func(id string) (*db.User, error) {
		if id == user.ID {
			return user, nil
		}
		return nil, db.ErrUserNotFound
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, _ = auth.Authenticate(req)
	}
}

// BenchmarkAuthenticator_NoAuthHeader measures the fastest failure path (missing header).
func BenchmarkAuthenticator_NoAuthHeader(b *testing.B) {
	auth, _, _, _ := setupAuthBenchmark(b)
	req := httptest.NewRequest("GET", "/", nil) // No header

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, _ = auth.Authenticate(req)
	}
}

// BenchmarkAuthenticator_InvalidFormat_NoBearer measures failure due to a missing "Bearer" prefix.
func BenchmarkAuthenticator_InvalidFormat_NoBearer(b *testing.B) {
	auth, _, _, _ := setupAuthBenchmark(b)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "invalid-token")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, _ = auth.Authenticate(req)
	}
}

// BenchmarkAuthenticator_MalformedToken measures failure due to a structurally invalid JWT.
func BenchmarkAuthenticator_MalformedToken(b *testing.B) {
	auth, _, _, _ := setupAuthBenchmark(b)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, _ = auth.Authenticate(req)
	}
}

// BenchmarkAuthenticator_ExpiredToken_UnverifiedCheck measures failure due to an expired token
// caught before the database lookup.
func BenchmarkAuthenticator_ExpiredToken_UnverifiedCheck(b *testing.B) {
	auth, _, cfg, user := setupAuthBenchmark(b)
	token := generateTestToken(b, user, cfg, -5*time.Minute) // Expired token
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, _ = auth.Authenticate(req)
	}
}

// BenchmarkAuthenticator_UserNotFound measures failure when the user ID from a valid token
// does not exist in the database.
func BenchmarkAuthenticator_UserNotFound(b *testing.B) {
	auth, mockDB, cfg, user := setupAuthBenchmark(b)
	token := generateTestToken(b, user, cfg, time.Hour)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Configure mock DB to return "user not found".
	mockDB.GetUserByIdFunc = func(id string) (*db.User, error) {
		return nil, db.ErrUserNotFound
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, _ = auth.Authenticate(req)
	}
}

// BenchmarkAuthenticator_InvalidSignature measures the critical failure path where the
// token's signature is invalid.
func BenchmarkAuthenticator_InvalidSignature(b *testing.B) {
	auth, mockDB, cfg, user := setupAuthBenchmark(b)
	token := generateTestToken(b, user, cfg, time.Hour)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Configure mock DB to return the valid user.
	mockDB.GetUserByIdFunc = func(id string) (*db.User, error) {
		if id == user.ID {
			return user, nil
		}
		return nil, db.ErrUserNotFound
	}

	// After generating the token, change the secret for verification.
	cfg.Jwt.AuthSecret = "a-different-secret-for-verification"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, _ = auth.Authenticate(req)
	}
}