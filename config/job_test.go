package config

import (
	"testing"
	"time"
)

func TestJobs_Get(t *testing.T) {
	jobs := Jobs{
		"acme_cert": {JobType: "acme_cert", Interval: Duration{Duration: time.Hour}, Activated: true},
		"backup":    {JobType: "backup", Interval: Duration{Duration: 24 * time.Hour}, Activated: true},
	}

	entry, found := jobs.Get("backup")
	if !found {
		t.Fatal("expected to find backup")
	}
	if entry.JobType != "backup" {
		t.Errorf("JobType: got %q, want %q", entry.JobType, "backup")
	}

	_, found = jobs.Get("missing")
	if found {
		t.Error("expected no entry for missing")
	}
}
