package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"time"
	"github.com/caasmo/restinpieces/db"
)

func (d *Db) Get() (*db.AcmeCert, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var cert db.AcmeCert
	var expiresStr string

	err := sqlitex.Exec(conn,
		`SELECT private_key, certificate, expires_at 
		FROM acme_certificates 
		ORDER BY created_at DESC 
		LIMIT 1;`,
		func(stmt *sqlite.Stmt) error {
			cert.Key = []byte(stmt.GetText("private_key"))
			cert.Certificate = []byte(stmt.GetText("certificate"))
			expiresStr = stmt.GetText("expires_at")
			return nil
		})

	if err != nil {
		return nil, fmt.Errorf("acme: failed to get cert: %w", err)
	}

	if len(cert.Key) == 0 || len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("acme: no certificate found")
	}

	cert.ExpiresAt, err = time.Parse(time.RFC3339, expiresStr)
	if err != nil {
		return nil, fmt.Errorf("acme: invalid expiration time: %w", err)
	}

	return &cert, nil
}
