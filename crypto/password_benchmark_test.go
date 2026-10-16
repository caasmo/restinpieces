package crypto

import "testing"

// These benchmarks keep the timing mitigation honest: VerifyPassword must cost
// the same for a valid record and for an unusable one — the error path burns
// one comparison against DummyPasswordHash. Run both and compare ns/op; they
// belong in the same range.
//
// The comparison is manual on purpose. Wall-clock assertions in unit tests are
// a documented flaky-test anti-pattern, and real timing verification is a
// black-box or statistical job (OWASP WSTG-IDNT-04, dudect-style t-tests over
// millions of samples) — never a CI gate.

func BenchmarkVerifyPassword_ValidRecord(b *testing.B) {
	hash := HashPassword("my_super_secret_password")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, _ = VerifyPassword("my_super_secret_password", hash)
	}
}

func BenchmarkVerifyPassword_UnusableRecord(b *testing.B) {
	salt := []byte("0123456789abcdef")
	key := []byte("0123456789abcdef0123456789abcdef")
	hash := record("v=19$m=0,t=2,p=1", salt, key)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, _ = VerifyPassword("my_super_secret_password", hash)
	}
}
