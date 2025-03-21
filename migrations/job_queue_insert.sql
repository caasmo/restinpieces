-- Insert test data into job_queue table in RFC3339 format
INSERT INTO job_queue (job_type, payload, status, created_at, updated_at, attempts, max_attempts, scheduled_for) VALUES
  ('email_verification', '{"email":"test1@example.com"}', 'pending', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 0, 3, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  ('email_verification', '{"email":"test2@example.com"}', 'failed', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 1, 3, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  ('email_verification', '{"email":"test3@example.com"}', 'pending', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 0, 3, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  ('password_reset', '{"email":"test4@example.com"}', 'pending', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 0, 3, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  ('account_reminder', '{"email":"test5@example.com"}', 'pending', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 0, 3, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  ('email_verification', '{"email":"test6@example.com"}', 'failed', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 2, 3, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  ('account_reminder', '{"email":"test7@example.com"}', 'pending', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 0, 3, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  ('password_reset', '{"email":"test8@example.com"}', 'pending', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 0, 3, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  ('email_verification', '{"email":"test9@example.com"}', 'pending', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 0, 3, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
  ('account_reminder', '{"email":"test10@example.com"}', 'pending', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 0, 3, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));
