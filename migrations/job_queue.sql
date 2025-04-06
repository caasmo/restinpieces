-- All time fields are UTC, RFC3339
-- we put unique in payload. This means if your job payload contains any maps
-- (map[string]interface{} or similar), the serialization might not be
-- deterministic across different json.Marshal calls with equivalent map contents.
-- To ensure deterministic serialization in Go, Use only structs (no maps) for
-- your job payloads. If payload too long, consider deterministic serialization
-- + hash
CREATE TABLE job_queue (
	-- the column is already defined as an INTEGER PRIMARY KEY, it's actually an alias for the rowid
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_type TEXT NOT NULL DEFAULT '',  -- Type of job (email_verification, password_reset, etc.)
    payload TEXT NOT NULL DEFAULT '',   -- JSON payload with job-specific data, but only the fields needed for uniqueness
    payload_extra TEXT NOT NULL DEFAULT '',   -- JSON payload with job-specific data, data not unique
    status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    attempts INTEGER NOT NULL DEFAULT 0, -- Number of processing attempts
    max_attempts INTEGER NOT NULL DEFAULT 0, -- Maximum retry attempts
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')), 
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')), 
    scheduled_for TEXT NOT NULL DEFAULT '', -- When to process this job
    locked_by TEXT NOT NULL DEFAULT '',     -- Worker ID that claimed this job
    locked_at TEXT NOT NULL DEFAULT '',     -- When the job was claimed
    completed_at TEXT NOT NULL DEFAULT '',  -- When the job was completed
    last_error TEXT NOT NULL DEFAULT ''     -- Last error message if failed
);

-- Create indexes separately
CREATE INDEX IF NOT EXISTS idx_job_queue_status_id ON job_queue(status, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_job_unique ON job_queue (payload, job_type);
