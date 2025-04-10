package crypto

import (
	"crypto/rand"
	"math/big"
)

const AlphanumericAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

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
