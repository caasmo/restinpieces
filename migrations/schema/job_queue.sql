-- All time fields are UTC, RFC3339
-- we put unique in payload. This means if your job payload contains any maps
-- (map[string]interface{} or similar), the serialization might not be
-- deterministic across different json.Marshal calls with equivalent map contents.
-- To ensure deterministic serialization in Go, Use only structs (no maps) for
-- your job payloads. If payload too long, consider deterministic serialization
-- + hash
CREATE TABLE job_queue (
	-- the comlumn is already defined as an INTEGER PRIMARY KEY, it's actually an alias for the rowid
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_type TEXT NOT NULL DEFAULT '',  -- Type of job (email_verification, password_reset, etc.)
    --priority INTEGER DEFAULT 1, -- Higher number = higher priority
    payload TEXT NOT NULL DEFAULT '',   -- JSON payload with job-specific data, but only the fields needed for uniqueness
    payload_extra TEXT NOT NULL DEFAULT '',   -- JSON payload with job-specific data, data not unique
    status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    attempts INTEGER NOT NULL DEFAULT 0, -- Number of processing attempts
    max_attempts INTEGER NOT NULL DEFAULT 0, -- Maximum retry attempts
	-- The parentheses around the strftime() function call are necessary for
    -- SQLite to recognize this as a valid default value expression
    -- format UTC, RFC3339. 
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')), 
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')), 
    scheduled_for TEXT NOT NULL DEFAULT '', -- When to process this job
    locked_by TEXT NOT NULL DEFAULT '',     -- Worker ID that claimed this job
    locked_at TEXT NOT NULL DEFAULT '',     -- When the job was claimed
    completed_at TEXT NOT NULL DEFAULT '',  -- When the job was completed
    last_error TEXT NOT NULL DEFAULT '',          -- Last error message if failed

	-- NEW fields for recurrence
	recurrent BOOLEAN NOT NULL DEFAULT FALSE,
	interval TEXT NOT NULL DEFAULT '', -- go duration
    
    -- Indexes for efficient querying (using CREATE INDEX instead of inline INDEX)
    UNIQUE (payload, job_type)
);

-- This means the combination of payload and job_type must be unique among all rows where status is either 'pending' or 'processing'.
-- This differs from a traditional table constraint defined with ALTER TABLE or in the table definition, but it functions as a constraint nonetheless. SQLite will prevent inserts or updates that would violate this uniqueness rule within the specified subset of rows.
-- CREATE UNIQUE INDEX idx_job_unique_active ON job_queue (payload, job_type) WHERE status NOT IN ('completed');
-- Create separate index statements
--CREATE INDEX idx_job_status ON job_queue (status, scheduled_for);
--CREATE INDEX idx_job_type ON job_queue (job_type, status);
-- CREATE INDEX idx_locked_by ON job_queue (locked_by);
CREATE INDEX idx_job_queue_status_id ON job_queue(status, id);
CREATE UNIQUE INDEX idx_job_unique ON job_queue (payload, job_type);
