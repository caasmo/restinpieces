package zombiezen

import (
	"context" // Added for Take context
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"strings" // Added for checking constraint errors
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// Get retrieves the latest ACME certificate based on issued_at timestamp.
func (d *Db) Get() (*db.AcmeCert, error) {
	conn, err := d.pool.Take(context.TODO()) // Use context.TODO() or pass a real context
	if err != nil {
		return nil, fmt.Errorf("acme: failed to get db connection: %w", err)
	}
	defer d.pool.Put(conn)

	var cert *db.AcmeCert // Initialize as nil

	err = sqlitex.Execute(conn,
		`SELECT 
			id, identifier, domains, certificate_chain, private_key, 
			issued_at, expires_at, last_renewal_attempt_at, created_at, updated_at
		FROM acme_certificates 
		ORDER BY issued_at DESC 
		LIMIT 1;`, // Order by issued_at to get the most recently issued cert
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				cert = &db.AcmeCert{
					ID:                     stmt.ColumnInt64(0), // id
					Identifier:             stmt.ColumnText(1),  // identifier
					Domains:                stmt.ColumnText(2),  // domains
					CertificateChain:       stmt.ColumnText(3),  // certificate_chain
					PrivateKey:             stmt.ColumnText(4),  // private_key
					IssuedAt:               stmt.ColumnText(5),  // issued_at
					ExpiresAt:              stmt.ColumnText(6),  // expires_at
					LastRenewalAttemptAt:   stmt.ColumnText(7),  // last_renewal_attempt_at
					CreatedAt:              stmt.ColumnText(8),  // created_at
					UpdatedAt:              stmt.ColumnText(9),  // updated_at
				}
				return nil
			},
		})

	if err != nil {
		return nil, fmt.Errorf("acme: failed to get cert: %w", err)
	}

	// If cert is still nil after query execution, no record was found
	if cert == nil {
		// Consider returning a specific error like db.ErrNotFound if needed downstream
		return nil, fmt.Errorf("acme: no certificate found")
	}

	return cert, nil
}

// Save inserts or updates an ACME certificate record based on the Identifier.
func (d *Db) Save(cert db.AcmeCert) error {
	conn, err := d.pool.Take(context.TODO()) // Use context.TODO() or pass a real context
	if err != nil {
		return fmt.Errorf("acme: failed to get db connection: %w", err)
	}
	defer d.pool.Put(conn)

	// Note: created_at and updated_at are handled by DB defaults/triggers
	// last_renewal_attempt_at is not set here, should be updated separately if needed.
	err = sqlitex.Execute(conn,
		`INSERT INTO acme_certificates (
			identifier, domains, certificate_chain, private_key, issued_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(identifier) DO UPDATE SET
			domains = excluded.domains,
			certificate_chain = excluded.certificate_chain,
			private_key = excluded.private_key,
			issued_at = excluded.issued_at,
			expires_at = excluded.expires_at,
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now');`,
		&sqlitex.ExecOptions{
			Args: []interface{}{
				cert.Identifier,
				cert.Domains,
				cert.CertificateChain,
				cert.PrivateKey,
				cert.IssuedAt,
				cert.ExpiresAt,
			},
		})

	if err != nil {
		// Check for unique constraint violation specifically
		// Note: Zombiezen might return a different error structure/message than crawshaw
		if sqlite.ErrCode(err) == sqlite.CONSTRAINT_UNIQUE || strings.Contains(err.Error(), "UNIQUE constraint failed") {
			// This specific error shouldn't happen with ON CONFLICT...DO UPDATE,
			// but checking just in case or for other potential constraints.
			return fmt.Errorf("acme save failed: %w: %w", db.ErrConstraintUnique, err)
		}
		return fmt.Errorf("acme: failed to save certificate for identifier %s: %w", cert.Identifier, err)
	}

	return nil
}
