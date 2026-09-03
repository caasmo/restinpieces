package crypto

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestRandomString(t *testing.T) {
	testCases := []struct {
		name     string
		length   int
		alphabet string
	}{
		{
			name:     "alphanumeric",
			length:   32,
			alphabet: AlphanumericAlphabet,
		},
		{
			name:     "pkce",
			length:   64,
			alphabet: pkceAlphabet,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := RandomString(tc.length, tc.alphabet)
			if len(s) != tc.length {
				t.Errorf("RandomString() length = %d, want %d", len(s), tc.length)
			}
			for _, char := range s {
				if !strings.ContainsRune(tc.alphabet, char) {
					t.Errorf("RandomString() contains invalid character: %c", char)
				}
			}
		})
	}
}

func TestNonce(t *testing.T) {
	n1 := Nonce()
	n2 := Nonce()

	if n1 == n2 {
		t.Errorf("Nonce() returned same value twice: %s", n1)
	}

	// 32 bytes base64 encoded should be 44 characters
	if len(n1) != 44 {
		t.Errorf("Nonce() length = %d, want 44", len(n1))
	}

	// Verify it's valid base64
	_, err := base64.StdEncoding.DecodeString(n1)
	if err != nil {
		t.Errorf("Nonce() returned invalid base64: %v", err)
	}
}

func TestRandomNumericOTP(t *testing.T) {
	otp := RandomNumericOTP()
	if len(otp) != 6 {
		t.Errorf("RandomNumericOTP() length = %d, want 6", len(otp))
	}

	for _, char := range otp {
		if char < '0' || char > '9' {
			t.Errorf("RandomNumericOTP() contains non-digit character: %c", char)
		}
	}
}

func TestRandomStringPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	RandomString(10, "")
}
