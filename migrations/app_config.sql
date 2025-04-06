-- Table for tracking configuration history with versioning
-- All time fields are UTC, RFC3339
CREATE TABLE IF NOT EXISTS app_config (
    -- id: Unique identifier for this specific version of the config
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- content: The actual configuration data (e.g., TOML string)
    content TEXT NOT NULL,

    -- format: The format of the content (e.g., 'toml', 'json')
    format TEXT NOT NULL DEFAULT 'toml',

    -- description: Optional text describing the change or version
    description TEXT,

    -- created_at: Timestamp when this version was inserted, used for ordering
    -- format UTC, RFC3339
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Create index separately to avoid trailing bytes in table creation
CREATE INDEX IF NOT EXISTS idx_app_config_created ON app_config(created_at DESC)

