package config

import (
	"testing"
	"time"
)

func TestJobs_Get(t *testing.T) {
	jobs := Jobs{
		"acme_cert": {JobType: "job_type_acme_cert", Interval: Duration{Duration: time.Hour}, Activated: true},
		"backup":    {JobType: "job_type_backup", Interval: Duration{Duration: 24 * time.Hour}, Activated: true},
	}

	entry, found := jobs.Get("job_type_backup")
	if !found {
		t.Fatal("expected to find job_type_backup")
	}
	if entry.JobType != "job_type_backup" {
		t.Errorf("JobType: got %q, want %q", entry.JobType, "job_type_backup")
	}

	_, found = jobs.Get("job_type_missing")
	if found {
		t.Error("expected no entry for job_type_missing")
	}
}
