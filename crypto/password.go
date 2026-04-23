package crypto

import "golang.org/x/crypto/bcrypt"

// CheckPassword compares a bcrypt hashed password with its possible plaintext equivalent
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateHash creates a bcrypt hash from a password using reasonable default cost
// bcrypt cost 10 is the historical default but is considered low by current
// standards. The cost is exponential — each increment doubles the work:
// Cost 10: ~100ms on a modern server
// Cost 12: ~400ms
// The tradeoff is direct: higher cost protects your stored hashes if the DB is
// ever leaked (slows offline cracking), but increases CPU load per login and
// worsens the DoS surface on your unauthenticated endpoint
// The current OWASP recommendation is cost 12 as a reasonable baseline for
// bcrypt. Cost 10 is becoming weak against modern GPU cracking rigs if your DB
// leaks.
func GenerateHash(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashedBytes), err
}

// DummyPasswordHash is a precomputed bcrypt hash used exclusively to equalise
// response time on the not-found path of credential handlers. The plaintext it
// was derived from is intentionally unknown and irrelevant — CheckPassword
// against this hash will always return false, which is the desired behaviour.
//
//      package main
//      
//      import (
//          "fmt"
//          "github.com/caasmo/restinpieces/crypto"
//      )
//      
//      func main() {
//          hash, err := crypto.GenerateHash("restinpieces-qeq99qt")
//          if err != nil {
//              panic(err)
//          }
//          fmt.Println(hash)
//      }
// 
// the dummy hash must be generated with the same cost as the production one
// generated cost 10
var DummyPasswordHash = "$2a$10$5kebOn7bqUSaEWKNMUzJ2elZSgL.od24R.S1TiFTUWXYapS2ILPDe"
