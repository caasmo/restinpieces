package crypto

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters used by every password hash.
//
// The numbers follow the password handling section of
// https://words.filippo.io/right-answers/.
const (
	argonTime    = 2
	argonMemory  = 65536 // KiB, Go counts KiB, so this is 64 MiB
	argonThreads = 1

	// argonMemoryPerThread is the smallest memory cost accepted per thread, in
	// KiB. argon2.IDKey silently raises memory to this floor, so a record below
	// it would verify at a cost other than the one it records.
	argonMemoryPerThread = 8

	// argonKeyLen is the derived key size in bytes. validateArgon2id enforces
	// argonKeyLenMin as the minimum and argonKeyLenMax as the maximum a stored
	// record may carry.
	argonKeyLen    = 32
	argonKeyLenMin = 16
	argonKeyLenMax = 64

	// argonSaltLen is 16 bytes because RFC 9106 recommends a 128-bit salt for
	// password hashing. validateArgon2id enforces this as the minimum and
	// argonSaltLenMax as the maximum a stored record may carry.
	argonSaltLen    = 16
	argonSaltLenMax = 32

	// argonTimeMax, argonMemoryMax and argonThreadsMax are the ceilings accepted
	// from a stored hash. Their values are independent of the settings above:
	// lowering argonTime, argonMemory or argonThreads must not make stronger
	// hashes already stored stop verifying.
	argonTimeMax    = 8
	argonMemoryMax  = 262144 // KiB, Go counts KiB, so this is 256 MiB
	argonThreadsMax = 4

	// argon2idFormat is the PHC string that stores a hash:
	// $argon2id$v=<version>$m=<memory>,t=<time>,p=<threads>$<salt>$<key>
	// salt and key are base64 RawStdEncoding (no padding).
	argon2idFormat = "$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s"
)

// ErrArgon2id is returned when a stored argon2id hash cannot be parsed or
// carries settings that cannot be used.
var ErrArgon2id = errors.New("argon2id error")

// HashPassword hashes a password with argon2id and returns the PHC string,
// for example $argon2id$v=19$m=65536,t=2,p=1$<salt>$<key>.
//
// A failure from the operating system entropy source panics, like the rest of
// this package.
func HashPassword(password string) string {
	b := randomBytes()
	salt := b[:argonSaltLen]

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf(argon2idFormat,
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key))
}

// argon2idHash is a parsed argon2id PHC string:
// $argon2id$v=<version>$m=<memory>,t=<time>,p=<threads>$<salt>$<key>.
type argon2idHash struct {
	version  uint32
	memory   uint32
	timeCost uint32
	threads  uint8
	salt     []byte
	key      []byte
}

// parseArgon2id reads a stored argon2id hash into an argon2idHash.
func parseArgon2id(hash string) (argon2idHash, error) {
	var parsed argon2idHash
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argon2idHash{}, fmt.Errorf("%w: invalid argon2id hash", ErrArgon2id)
	}

	_, err := fmt.Sscanf(parts[2], "v=%d", &parsed.version)
	if err != nil {
		return argon2idHash{}, fmt.Errorf("%w: invalid argon2id version: %w", ErrArgon2id, err)
	}

	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &parsed.memory, &parsed.timeCost, &parsed.threads)
	if err != nil {
		return argon2idHash{}, fmt.Errorf("%w: invalid argon2id parameters: %w", ErrArgon2id, err)
	}

	parsed.salt, err = base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return argon2idHash{}, fmt.Errorf("%w: invalid argon2id salt: %w", ErrArgon2id, err)
	}
	parsed.key, err = base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return argon2idHash{}, fmt.Errorf("%w: invalid argon2id key: %w", ErrArgon2id, err)
	}

	// Sscanf ignores trailing junk and any prefix inside the fields, so the
	// record must be exactly the canonical form of what was parsed.
	canonical := fmt.Sprintf(argon2idFormat,
		parsed.version, parsed.memory, parsed.timeCost, parsed.threads,
		base64.RawStdEncoding.EncodeToString(parsed.salt),
		base64.RawStdEncoding.EncodeToString(parsed.key))
	if canonical != hash {
		return argon2idHash{}, fmt.Errorf("%w: invalid argon2id hash", ErrArgon2id)
	}

	return parsed, nil
}

// validateArgon2id rejects parsed settings that would make argon2.IDKey panic
// or would let any password match. It also rejects cost settings outside their
// allowed range and a salt or key outside its recommended window.
func validateArgon2id(parsed argon2idHash) error {
	if parsed.version != argon2.Version {
		return fmt.Errorf("%w: unsupported argon2id version", ErrArgon2id)
	}
	if parsed.memory < argonMemoryPerThread*uint32(parsed.threads) {
		return fmt.Errorf("%w: argon2id memory is below %d KiB per thread", ErrArgon2id, argonMemoryPerThread)
	}
	if parsed.memory > argonMemoryMax {
		return fmt.Errorf("%w: argon2id memory is above the ceiling", ErrArgon2id)
	}
	if parsed.timeCost < 1 {
		return fmt.Errorf("%w: argon2id time is below 1", ErrArgon2id)
	}
	if parsed.timeCost > argonTimeMax {
		return fmt.Errorf("%w: argon2id time is above the ceiling", ErrArgon2id)
	}
	if parsed.threads < 1 {
		return fmt.Errorf("%w: argon2id threads is below 1", ErrArgon2id)
	}
	if parsed.threads > argonThreadsMax {
		return fmt.Errorf("%w: argon2id threads is above the ceiling", ErrArgon2id)
	}
	if len(parsed.salt) < argonSaltLen {
		return fmt.Errorf("%w: argon2id salt is shorter than the recommended %d bytes", ErrArgon2id, argonSaltLen)
	}
	if len(parsed.salt) > argonSaltLenMax {
		return fmt.Errorf("%w: argon2id salt is longer than the ceiling of %d bytes", ErrArgon2id, argonSaltLenMax)
	}
	if len(parsed.key) < argonKeyLenMin || len(parsed.key) > argonKeyLenMax {
		return fmt.Errorf("%w: argon2id key length out of range", ErrArgon2id)
	}

	return nil
}

// VerifyPassword reports whether the password matches the stored hash. Only
// argon2id hashes are accepted; the parameters come from the hash itself.
//
// False with a nil error means the password does not match. A non-nil error
// means the stored record is unusable — malformed, or carrying settings we
// refuse — which is not a wrong password: callers must log it instead of
// treating it as a failed login.
//
// An unusable record still costs one comparison, run against DummyPasswordHash,
// so the answer takes as long as a real check and the response time cannot
// reveal the unusable record.
func VerifyPassword(password, hash string) (bool, error) {
	matched, err := verifyArgon2id(password, hash)
	if err != nil {
		_, _ = verifyArgon2id(password, DummyPasswordHash)
	}

	return matched, err
}

// verifyArgon2id parses, validates and compares one stored record. It does not
// equalise the time spent on an unusable record — VerifyPassword does.
func verifyArgon2id(password, hash string) (bool, error) {
	parsed, err := parseArgon2id(hash)
	if err != nil {
		return false, err
	}

	err = validateArgon2id(parsed)
	if err != nil {
		return false, err
	}

	candidate := argon2.IDKey([]byte(password), parsed.salt, parsed.timeCost, parsed.memory, parsed.threads, uint32(len(parsed.key)))

	return subtle.ConstantTimeCompare(candidate, parsed.key) == 1, nil
}

// DummyPasswordHash is a precomputed argon2id hash used exclusively to
// equalise response time on the not-found path of credential handlers. The
// plaintext it was derived from is intentionally unknown and irrelevant —
// VerifyPassword against this hash will always return false, which is the
// desired behaviour.
//
// It must be generated with the same parameters as the production one.
const DummyPasswordHash = "$argon2id$v=19$m=65536,t=2,p=1$BGC5X0/e2ewl9Bmdugi5JA$aPn8YO4n9QlvAFdqRClRoFVRa867jXcvCFk56riHPtc"
