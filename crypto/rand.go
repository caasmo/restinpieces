package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
)

const AlphanumericAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func RandomNumericOTP() string {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func RandomString(length int, alphabet string) string {

	b := make([]byte, length)

	// The crypto/rand.Int() function specifically requires a *big.Int
	// parameter to define the upper bound of the random number.
	// big.Int is used in cryptography contexts because it can handle
	// arbitrarily large integers with precision, which is important for
	// cryptographic operations.
	max := big.NewInt(int64(len(alphabet)))

	for i := range b {
		// The first parameter rand is an io.Reader that provides the random bytes.
		// Here, cryptoRand.Reader is used, which is a globally shared source of
		// cryptographic randomness.
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// The Go documentation specifically states that the function
			// returns an error if the random number generator fails, which is
			// a critical security condition. The error should never be
			// ignored, as doing so could lead to using predictable values in
			// security-sensitive contexts.
			panic(err)
		}
		b[i] = alphabet[n.Int64()]
	}

	return string(b)
}

// Nonce generates a cryptographically secure random nonce suitable for use
// in Content Security Policy (CSP) script-src and style-src directives.
//
// # Standards and Requirements
//
// ## W3C Content Security Policy Level 2 (https://www.w3.org/TR/CSP2/)
// ## W3C Content Security Policy Level 3 (https://www.w3.org/TR/CSP3/)
//
//   - Section 2.4 defines a nonce as a base64-encoded cryptographic random value.
//   - The nonce attribute value in HTML must be valid base64; alphanumeric or hex
//     encodings are not conforming.
//   - The same nonce value must appear in both the CSP response header and the
//     corresponding <script> or <style> tag attribute:
//     Header:  Content-Security-Policy: script-src 'nonce-<value>'
//     Element: <script nonce="<value>">
//
// ## OWASP Content Security Policy Cheat Sheet
// (https://cheatsheetseries.owasp.org/cheatsheets/Content_Security_Policy_Cheat_Sheet.html)
//
//   - Minimum 128 bits of entropy required; 256 bits recommended.
//   - Must be generated fresh for every HTTP response. A nonce reused across
//     responses is semantically equivalent to no nonce — it collapses into a
//     static allowlist that any attacker can predict.
//   - Must never appear in URLs, logs, or any location other than the CSP header
//     and the matching HTML attribute.
//
// ## NIST SP 800-90A (https://csrc.nist.gov/publications/detail/sp/800-90a/rev-1/final)
//
//   - Mandates a CSPRNG (Cryptographically Secure Pseudo-Random Number Generator)
//     as the source of randomness. crypto/rand.Read() satisfies this requirement
//     by reading from the OS entropy source (/dev/urandom on Linux, CNG on Windows).
//
// # Implementation Notes
//
// 32 bytes (256 bits) are read directly from crypto/rand into a raw byte slice.
// This avoids big.Int, which is designed for arithmetic on arbitrarily large
// integers and carries unnecessary overhead when the goal is simply filling a
// buffer with random bytes.
//
// base64.StdEncoding produces a 44-character string from 32 bytes, which is
// the encoding required by the CSP specification.
//
// A failure from crypto/rand.Read is treated as an unrecoverable condition and
// causes a panic. This is consistent with the rest of this package and with Go
// stdlib conventions for CSPRNG failures: a broken entropy source means the
// security guarantees of the entire system are void, and continuing execution
// would be more dangerous than crashing.
func Nonce() string {
	b := make([]byte, 32) // 256 bits
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}
