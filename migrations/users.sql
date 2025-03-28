-- All time fields are UTC, RFC3339
-- When using the BOOLEAN type in SQLite, the data
-- is stored as 0 or 1 (as INTEGER), and this is the standard SQLite behavior. 
-- In SQLite, the BOOLEAN type is simply an alias for INTEGER,
-- verified` BOOLEAN DEFAULT FALSE NOT NULL is an alias for `verified` INTEGER DEFAULT 0 NOT NULL
-- sqlite package like crawshaw will automatically convert go boolean types to integer 0 or 1 (writes)
CREATE TABLE `users`(
  `id` TEXT PRIMARY KEY DEFAULT('r'||lower(hex(randomblob(7)))) NOT NULL,
  `name` TEXT DEFAULT '' NOT NULL,
  `password` TEXT DEFAULT '' NOT NULL,
  `verified` BOOLEAN DEFAULT FALSE NOT NULL,
  `oauth2` BOOLEAN DEFAULT FALSE NOT NULL,
  `externalAuth` TEXT DEFAULT '' NOT NULL,
  `avatar` TEXT DEFAULT '' NOT NULL,
  `email` TEXT DEFAULT '' NOT NULL UNIQUE, -- Ensures email uniqueness for user accounts
  `emailVisibility` BOOLEAN DEFAULT FALSE NOT NULL,
  `created` TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  `updated` TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
