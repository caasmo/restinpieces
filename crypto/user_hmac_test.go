package crypto

import "testing"

func TestUserMac(t *testing.T) {
	userID := "user_123456789"
	secret := "a_very_long_and_secure_server_secret_32_bytes"

	mac := GenerateUserMac(userID, secret)

	// Verify length (16 hex chars = 64 bits)
	if len(mac) != 16 {
		t.Errorf("GenerateUserMac() length = %d, want 16", len(mac))
	}

	// Test 1: Successful verification
	if !VerifyUserMac(userID, mac, secret) {
		t.Errorf("VerifyUserMac() = false, want true for valid MAC")
	}

	// Test 2: Mismatched userID
	if VerifyUserMac("different_user", mac, secret) {
		t.Error("VerifyUserMac() = true, want false for mismatched userID")
	}

	// Test 3: Mismatched secret
	if VerifyUserMac(userID, mac, "wrong_secret_is_also_long_enough") {
		t.Error("VerifyUserMac() = true, want false for mismatched secret")
	}

	// Test 4: Invalid MAC length
	if VerifyUserMac(userID, mac[:15], secret) {
		t.Error("VerifyUserMac() = true, want false for short MAC")
	}
	if VerifyUserMac(userID, mac+"a", secret) {
		t.Error("VerifyUserMac() = true, want false for long MAC")
	}

	// Test 5: Empty values
	if VerifyUserMac("", GenerateUserMac("", secret), secret) == false {
		t.Error("VerifyUserMac() = false, want true for empty userID")
	}
}

func TestUserMacUniqueness(t *testing.T) {
	secret := "secure_secret_long_enough_to_be_safe"
	mac1 := GenerateUserMac("user1", secret)
	mac2 := GenerateUserMac("user2", secret)

	if mac1 == mac2 {
		t.Errorf("GenerateUserMac() produced identical MACs for different userIDs: %s", mac1)
	}
}

func TestUserMacConsistency(t *testing.T) {
	userID := "user123"
	secret := "secure_secret_long_enough_to_be_safe"
	
	mac1 := GenerateUserMac(userID, secret)
	mac2 := GenerateUserMac(userID, secret)

	if mac1 != mac2 {
		t.Errorf("GenerateUserMac() is non-deterministic: %s != %s", mac1, mac2)
	}
}
