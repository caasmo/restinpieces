-- All time fields are UTC, RFC3339
CREATE TABLE acme_certificates (
    -- Primary Key
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Identifier & Domains
    -- A unique identifier for this certificate config (e.g., 'main-app', 'api.example.com').
    -- Useful if you manage certs for multiple distinct services/domain groups.
    identifier TEXT NOT NULL UNIQUE,
    -- JSON array of all domain names covered by this certificate (SANs included).
    -- Example: '["example.com", "www.example.com", "api.example.com"]'
    domains TEXT NOT NULL,

    -- Certificate Data (PEM Encoded)
    -- The full certificate chain (leaf certificate + intermediates) provided by Let's Encrypt.
    certificate_chain TEXT NOT NULL,
    -- The private key corresponding to the certificate.
    -- !! SECURITY WARNING: Store this securely. See notes below. !!
    private_key TEXT NOT NULL,

    -- Timestamps (Using ISO8601 format for clarity, ensure UTC)
    -- When the current certificate was successfully issued. Format: 'YYYY-MM-DDTHH:MM:SSZ'
    issued_at TEXT NOT NULL,
    -- When the current certificate expires. Format: 'YYYY-MM-DDTHH:MM:SSZ'
    expires_at TEXT NOT NULL,
    -- Timestamp of the last time a renewal attempt was made for this identifier.
    last_renewal_attempt_at TEXT,
    -- Timestamp when this record was created in the database.
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
     -- Timestamp when this record was last updated.
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

