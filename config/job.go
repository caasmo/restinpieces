package config

// Jobs lists the jobs that run on a schedule. Each map key is a name you
// choose, for example "acme_cert". The job handler is named in
// JobEntry.JobType, not in the key.
type Jobs map[string]JobEntry

// JobEntry describes one job that runs on a schedule.
//
// Activated false turns the job off: the scheduler stops adding runs to the
// queue. A run that is already queued still runs, and no new run is added
// after it.
type JobEntry struct {
	// JobType is the handler name the application registered with
	// Server.AddJobHandler, for example "acme_cert".
	JobType string `toml:"job_type" comment:"Job handler type registered by the app (e.g. 'acme_cert')"`

	// Interval is how often the job runs, for example "1h". The next run is
	// scheduled one interval after the completed run's scheduled time.
	Interval Duration `toml:"interval" comment:"How often the job runs (e.g. '1h')"`

	// Activated turns the job on or off.
	Activated bool `toml:"activated" comment:"Activate this job"`
}

// Get returns the entry with the given job type. The second result is false
// when no entry has that type. Two entries must not use the same type
// (ValidateJobs rejects that), so the first match is the only match.
func (j Jobs) Get(jobType string) (JobEntry, bool) {
	for _, entry := range j {
		if entry.JobType == jobType {
			return entry, true
		}
	}
	return JobEntry{}, false
}
