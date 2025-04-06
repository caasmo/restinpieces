-- Table for tracking configuration history with versioning
-- All time fields are UTC, RFC3339
CREATE TABLE IF NOT EXISTS config_history (
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
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    -- Index for efficient querying by creation time
    INDEX idx_config_history_created (created_at DESC)
);

-- Create a trigger to automatically maintain a limited history
-- Keeps only the most recent 100 versions (adjust as needed)
CREATE TRIGGER IF NOT EXISTS trim_config_history
AFTER INSERT ON config_history
BEGIN
    DELETE FROM config_history
    WHERE id NOT IN (
        SELECT id FROM config_history
        ORDER BY created_at DESC
        LIMIT 100
    );
END;
