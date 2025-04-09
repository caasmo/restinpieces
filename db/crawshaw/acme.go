package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"github.com/caasmo/restinpieces/db"
)

func (d *Db) Get() (*db.AcmeCert, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var cert db.AcmeCert

	err := sqlitex.Exec(conn,
		`SELECT private_key, certificate_chain
		FROM acme_certificates 
		ORDER BY created_at DESC 
		LIMIT 1;`,
		func(stmt *sqlite.Stmt) error {
			cert.Key = []byte(stmt.GetText("private_key"))
			cert.Certificate = []byte(stmt.GetText("certificate_chain"))
			return nil
		})

	if err != nil {
		return nil, fmt.Errorf("acme: failed to get cert: %w", err)
	}

	if len(cert.Key) == 0 || len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("acme: no certificate found")
	}

	return &cert, nil
}
