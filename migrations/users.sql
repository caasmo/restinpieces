CREATE TABLE `users`(
  `avatar` TEXT DEFAULT '' NOT NULL,
  `created` TEXT DEFAULT '' NOT NULL,
  `email` TEXT DEFAULT '' NOT NULL,
  `emailVisibility` BOOLEAN DEFAULT FALSE NOT NULL,
  `id` TEXT PRIMARY KEY DEFAULT('r'||lower(hex(randomblob(7)))) NOT NULL,
  `name` TEXT DEFAULT '' NOT NULL,
  `password` TEXT DEFAULT '' NOT NULL,
  `tokenKey` TEXT DEFAULT '' NOT NULL,
  `updated` TEXT DEFAULT '' NOT NULL,
  `verified` BOOLEAN DEFAULT FALSE NOT NULL
);
CREATE UNIQUE INDEX `idx_tokenKey__pb_users_auth_` ON `users`(`tokenKey`);
CREATE UNIQUE INDEX `idx_email__pb_users_auth_` ON `users`(
  `email`
) WHERE `email` != '';
