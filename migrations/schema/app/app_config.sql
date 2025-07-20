-- Table for tracking configuration history with versioning
-- All time fields are UTC, RFC3339
CREATE TABLE IF NOT EXISTS app_config (
    -- id: Unique identifier for this specific version of the config
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- scope: Defines the category or area this configuration applies to (e.g., 'application', 'plugin_x')
    -- Must be explicitly provided on insert.
    scope TEXT NOT NULL,

    -- content: The actual configuration data (e.g., TOML string)
    content BLOB NOT NULL,

    -- format: The format of the content (e.g., 'toml', 'json')
    format TEXT NOT NULL DEFAULT 'toml',

    -- description: Optional text describing the change or version
    description TEXT,

    -- created_at: Timestamp when this version was inserted, used for ordering
    -- format UTC, RFC3339Nano
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now', 'subsec', 'nanoseconds'))
);

-- Create index separately to avoid trailing bytes in table creation
CREATE INDEX IF NOT EXISTS idx_app_config_created ON app_config(created_at DESC);

