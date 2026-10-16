package crypto

import (
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
)

func TestVerifyPasswordRoundTrip(t *testing.T) {
	password := "my_super_secret_password"
	hash := HashPassword(password)

	matched, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !matched {
		t.Error("VerifyPassword() = false, want true")
	}

	matched, err = VerifyPassword("wrong_password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if matched {
		t.Error("VerifyPassword() = true, want false")
	}
}

func TestVerifyPasswordRejectsBadRecords(t *testing.T) {
	salt := []byte("0123456789abcdef")
	key := []byte("0123456789abcdef0123456789abcdef")
	validParams := "v=19$m=65536,t=2,p=1"

	testCases := []struct {
		name string
		hash string
	}{
		{
			name: "time is zero",
			hash: record("v=19$m=65536,t=0,p=1", salt, key),
		},
		{
			name: "threads is zero",
			hash: record("v=19$m=65536,t=2,p=0", salt, key),
		},
		{
			name: "threads above 255",
			hash: record("v=19$m=65536,t=2,p=256", salt, key),
		},
		{
			name: "memory is zero",
			hash: record("v=19$m=0,t=2,p=1", salt, key),
		},
		{
			name: "version with trailing junk",
			hash: record("v=19x$m=65536,t=2,p=1", salt, key),
		},
		{
			name: "threads with leading zero",
			hash: record("v=19$m=65536,t=2,p=01", salt, key),
		},
		{
			name: "threads with plus sign",
			hash: record("v=19$m=65536,t=2,p=+1", salt, key),
		},
		{
			name: "leading junk prefix",
			hash: "junk" + record(validParams, salt, key),
		},
		{
			name: "trailing newline",
			hash: record(validParams, salt, key) + "\n",
		},
		{
			name: "one byte key",
			hash: record(validParams, salt, []byte{1}),
		},
		{
			name: "three byte salt",
			hash: record(validParams, []byte{1, 2, 3}, key),
		},
		{
			name: "memory above the ceiling",
			hash: record(fmt.Sprintf("v=19$m=%d,t=2,p=1", argonMemoryMax+1), salt, key),
		},
		{
			name: "time above the ceiling",
			hash: record(fmt.Sprintf("v=19$m=65536,t=%d,p=1", argonTimeMax+1), salt, key),
		},
		{
			name: "threads above the ceiling",
			hash: record(fmt.Sprintf("v=19$m=65536,t=2,p=%d", argonThreadsMax+1), salt, key),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matched, err := VerifyPassword("my_super_secret_password", tc.hash)
			if !errors.Is(err, ErrArgon2id) {
				t.Errorf("VerifyPassword() error = %v, want %v", err, ErrArgon2id)
			}
			if matched {
				t.Error("VerifyPassword() = true, want false")
			}
		})
	}
}

// record assembles an argon2id PHC string from raw fields, so a test case can
// control the parameters and the salt and key lengths.
func record(params string, salt, key []byte) string {
	return "$argon2id$" + params + "$" +
		base64.RawStdEncoding.EncodeToString(salt) + "$" +
		base64.RawStdEncoding.EncodeToString(key)
}

func TestDummyPasswordHash(t *testing.T) {
	// The dummy hash must parse and reject every password without an error:
	// the not-found path of the credential handlers depends on it.
	inputs := []string{"password123", "", " "}

	for _, input := range inputs {
		matched, err := VerifyPassword(input, DummyPasswordHash)
		if err != nil {
			t.Errorf("VerifyPassword(%q, DummyPasswordHash) error = %v, want nil", input, err)
		}
		if matched {
			t.Errorf("VerifyPassword(%q, DummyPasswordHash) = true, want false", input)
		}
	}
}
