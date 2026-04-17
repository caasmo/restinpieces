package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func HashOtp(otp, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(otp))
	return hex.EncodeToString(h.Sum(nil))
}
