-- Logs table stores application logs in structured format for querying and analysis
-- Uses RFC 3339 format with milliseconds for timestamps (compatible with Go's time.RFC3339Nano)
CREATE TABLE IF NOT EXISTS logs (
    -- Unique identifier using random 7 bytes prefixed with 'r' for better readability
    -- Example: "r4e3a7d9b2c1f0"
    id TEXT PRIMARY KEY DEFAULT ('r'||lower(hex(randomblob(7)))) NOT NULL,
    
    -- Numeric log level matching slog.Level values:
    -- -4: Debug
    --  0: Info  
    --  4: Warn
    --  8: Error
    level INTEGER DEFAULT 0 NOT NULL,
    
    -- Human-readable log message
    message TEXT DEFAULT "" NOT NULL,
    
    -- Structured log attributes in JSON format
    -- Contains additional context like error details, request IDs etc.
    data JSON DEFAULT "{}" NOT NULL,
    
    -- Creation timestamp in RFC 3339 format with milliseconds
    -- Example: "2025-05-15T16:42:03.123Z"
    created TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ')) NOT NULL
);

-- Index for filtering logs by severity level
CREATE INDEX IF NOT EXISTS idx_logs_level ON logs (level);

-- Index for searching log messages (supports prefix searches)
CREATE INDEX IF NOT EXISTS idx_logs_message ON logs (message);
