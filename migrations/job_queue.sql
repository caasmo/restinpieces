CREATE TABLE job_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_type TEXT NOT NULL,  -- Type of job (email_verification, password_reset, etc.)
    --priority INTEGER DEFAULT 1, -- Higher number = higher priority
    payload TEXT NOT NULL,   -- JSON payload with job-specific data
    status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    attempts INTEGER NOT NULL DEFAULT 0, -- Number of processing attempts
    max_attempts INTEGER NOT NULL DEFAULT 3, -- Maximum retry attempts
    created_at TEXT NOT NULL DEFAULT (datetime('now')), -- ISO8601 string format
    updated_at TEXT NOT NULL DEFAULT (datetime('now')), -- ISO8601 string format
    scheduled_for TEXT NOT NULL DEFAULT (datetime('now')), -- When to process this job
    locked_by TEXT,          -- Worker ID that claimed this job
    locked_at TEXT,          -- When the job was claimed
    completed_at TEXT,       -- When the job was completed
    last_error TEXT          -- Last error message if failed
    
    -- Indexes for efficient querying (using CREATE INDEX instead of inline INDEX)
);

-- Create separate index statements
CREATE INDEX idx_job_status ON job_queue (status, scheduled_for);
CREATE INDEX idx_job_type ON job_queue (job_type, status);
--CREATE INDEX idx_locked_by ON job_queue (locked_by);
