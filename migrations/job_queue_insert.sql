-- Insert test data into job_queue table
INSERT INTO job_queue (job_type, payload, status, created_at, updated_at, attempts, max_attempts, scheduled_for) VALUES
  ('email_verification', '{"email":"test1@example.com"}', 'pending', datetime('now'), datetime('now'), 0, 3, datetime('now')),
  ('email_verification', '{"email":"test2@example.com"}', 'failed', datetime('now'), datetime('now'), 1, 3, datetime('now')),
  ('email_verification', '{"email":"test3@example.com"}', 'pending', datetime('now'), datetime('now'), 0, 3, datetime('now')),
  ('password_reset', '{"email":"test4@example.com"}', 'pending', datetime('now'), datetime('now'), 0, 3, datetime('now')),
  ('account_reminder', '{"email":"test5@example.com"}', 'pending', datetime('now'), datetime('now'), 0, 3, datetime('now')),
  ('email_verification', '{"email":"test6@example.com"}', 'failed', datetime('now'), datetime('now'), 2, 3, datetime('now')),
  ('account_reminder', '{"email":"test7@example.com"}', 'pending', datetime('now'), datetime('now'), 0, 3, datetime('now')),
  ('password_reset', '{"email":"test8@example.com"}', 'pending', datetime('now'), datetime('now'), 0, 3, datetime('now')),
  ('email_verification', '{"email":"test9@example.com"}', 'pending', datetime('now'), datetime('now'), 0, 3, datetime('now')),
  ('account_reminder', '{"email":"test10@example.com"}', 'pending', datetime('now'), datetime('now'), 0, 3, datetime('now'));
