package crypto

import (
	"encoding/base64"
	"testing"
)

func TestID(t *testing.T) {
	id1 := ID()
	id2 := ID()

	if id1 == id2 {
		t.Errorf("ID() returned same value twice: %s", id1)
	}

	// 32 bytes base64url encoded without padding should be 43 characters
	if len(id1) != 43 {
		t.Errorf("ID() length = %d, want 43", len(id1))
	}

	_, err := base64.RawURLEncoding.DecodeString(id1)
	if err != nil {
		t.Errorf("ID() returned invalid base64url: %v", err)
	}
}

func TestCSPNonce(t *testing.T) {
	n1 := CSPNonce()
	n2 := CSPNonce()

	if n1 == n2 {
		t.Errorf("CSPNonce() returned same value twice: %s", n1)
	}

	// 32 bytes base64 encoded should be 44 characters
	if len(n1) != 44 {
		t.Errorf("CSPNonce() length = %d, want 44", len(n1))
	}

	// Verify it's valid base64
	_, err := base64.StdEncoding.DecodeString(n1)
	if err != nil {
		t.Errorf("CSPNonce() returned invalid base64: %v", err)
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
