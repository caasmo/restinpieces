-- All time fields are UTC, RFC3339
CREATE TABLE `users`(
  `avatar` TEXT DEFAULT '' NOT NULL,
  `created` TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  `email` TEXT DEFAULT '' NOT NULL,
  `emailVisibility` BOOLEAN DEFAULT FALSE NOT NULL,
  `id` TEXT PRIMARY KEY DEFAULT('r'||lower(hex(randomblob(7)))) NOT NULL,
  `name` TEXT DEFAULT '' NOT NULL,
  `password` TEXT DEFAULT '' NOT NULL,
  `tokenKey` TEXT DEFAULT '' NOT NULL,
  `updated` TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  `verified` BOOLEAN DEFAULT FALSE NOT NULL
);
CREATE UNIQUE INDEX `idx_tokenKey__pb_users_auth_` ON `users`(`tokenKey`);
CREATE UNIQUE INDEX `idx_email__pb_users_auth_` ON `users`(
  `email`
) WHERE `email` != '';
