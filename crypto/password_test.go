package crypto

import "testing"

func TestGenerateAndCheckPassword(t *testing.T) {
	password := "my_super_secret_password"

	hash, err := GenerateHash(password)
	if err != nil {
		t.Fatalf("GenerateHash() error = %v", err)
	}

	if !CheckPassword(password, hash) {
		t.Error("CheckPassword() = false, want true")
	}

	if CheckPassword("wrong_password", hash) {
		t.Error("CheckPassword() = true, want false")
	}
}