CREATE TABLE logs (
    id TEXT PRIMARY KEY DEFAULT ('r'||lower(hex(randomblob(7)))) NOT NULL,
    level INTEGER DEFAULT 0 NOT NULL,
    message TEXT DEFAULT "" NOT NULL,
    data JSON DEFAULT "{}" NOT NULL,
    created TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL
);

CREATE INDEX idx_logs_level ON logs (level);
CREATE INDEX idx_logs_message ON logs (message);
