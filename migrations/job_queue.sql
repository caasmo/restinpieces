-- All time fields are UTC, RFC3339
CREATE TABLE job_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_type TEXT NOT NULL DEFAULT '',  -- Type of job (email_verification, password_reset, etc.)
    --priority INTEGER DEFAULT 1, -- Higher number = higher priority
    payload TEXT NOT NULL DEFAULT '',   -- JSON payload with job-specific data
    status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    attempts INTEGER NOT NULL DEFAULT 0, -- Number of processing attempts
    max_attempts INTEGER NOT NULL DEFAULT 3, -- Maximum retry attempts
	-- The parentheses around the strftime() function call are necessary for
    -- SQLite to recognize this as a valid default value expression
	-- format UTC, RFC3339. 
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')), 
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')), 
    scheduled_for TEXT NOT NULL DEFAULT '', -- When to process this job
    locked_by TEXT,          -- Worker ID that claimed this job
    locked_at TEXT NOT NULL DEFAULT '',          -- When the job was claimed
    completed_at TEXT NOT NULL DEFAULT '',       -- When the job was completed
    last_error TEXT NOT NULL DEFAULT '',          -- Last error message if failed
    
    -- Indexes for efficient querying (using CREATE INDEX instead of inline INDEX)
    UNIQUE (payload, job_type)
);

-- Create separate index statements
CREATE UNIQUE INDEX idx_job_unique ON job_queue (payload, job_type);
CREATE INDEX idx_job_status ON job_queue (status, scheduled_for);
CREATE INDEX idx_job_type ON job_queue (job_type, status);
--CREATE INDEX idx_locked_by ON job_queue (locked_by);
