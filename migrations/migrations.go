package migrations

import _ "embed"

//go:embed users.sql
var UsersSchema string

//go:embed  job_queue.sql
var JobQueueSchema string
