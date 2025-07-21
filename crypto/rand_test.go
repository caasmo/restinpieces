package crypto

import (
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

func TestRandomStringPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	RandomString(10, "")
}