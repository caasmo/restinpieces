package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"strings" // Added for checking constraint errors
)

// Get retrieves the latest ACME certificate based on issued_at timestamp.
func (d *Db) Get() (*db.AcmeCert, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var cert *db.AcmeCert // Initialize as nil

	err := sqlitex.Exec(conn,
		`SELECT 
			id, identifier, domains, certificate_chain, private_key, 
			issued_at, expires_at, last_renewal_attempt_at, created_at, updated_at
		FROM acme_certificates 
		ORDER BY issued_at DESC 
		LIMIT 1;`, // Order by issued_at to get the most recently issued cert
		func(stmt *sqlite.Stmt) error {
			cert = &db.AcmeCert{
				ID:                     stmt.GetInt64("id"),
				Identifier:             stmt.GetText("identifier"),
				Domains:                stmt.GetText("domains"),
				CertificateChain:       stmt.GetText("certificate_chain"),
				PrivateKey:             stmt.GetText("private_key"),
				IssuedAt:               stmt.GetText("issued_at"),
				ExpiresAt:              stmt.GetText("expires_at"),
				LastRenewalAttemptAt:   stmt.GetText("last_renewal_attempt_at"), // Handle potential null
				CreatedAt:              stmt.GetText("created_at"),
				UpdatedAt:              stmt.GetText("updated_at"),
			}
			return nil
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
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	// Note: created_at and updated_at are handled by DB defaults/triggers
	// last_renewal_attempt_at is not set here, should be updated separately if needed.
	err := sqlitex.Exec(conn,
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
		nil, // No result function needed for INSERT/UPDATE
		cert.Identifier,
		cert.Domains,
		cert.CertificateChain,
		cert.PrivateKey,
		cert.IssuedAt,
		cert.ExpiresAt,
	)

	if err != nil {
		// Check for unique constraint violation specifically
		if strings.Contains(err.Error(), "UNIQUE constraint failed: acme_certificates.identifier") {
			// This specific error shouldn't happen with ON CONFLICT...DO UPDATE,
			// but checking just in case or for other potential constraints.
			return fmt.Errorf("acme save failed: %w: %w", db.ErrConstraintUnique, err)
		}
		return fmt.Errorf("acme: failed to save certificate for identifier %s: %w", cert.Identifier, err)
	}

	return nil
}
