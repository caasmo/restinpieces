package dbtest

import (
	"testing"
	"time"

	"github.com/caasmo/restinpieces/db"
)

type QueueAdminSuite struct {
	Db interface {
		db.DbQueue
		db.DbQueueAdmin
	}
}

func (s QueueAdminSuite) TestListJobs(t *testing.T) {
	testDB := s.Db

	t.Run("List with no limit", func(t *testing.T) {
		baselineJobs, _ := testDB.ListJobs(0)
		baseline := len(baselineJobs)
		// Insert jobs with a slight delay to ensure distinct creation times for ordering
		if err := testDB.InsertJob(db.Job{JobType: "job1"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}
		time.Sleep(1 * time.Millisecond)
		if err := testDB.InsertJob(db.Job{JobType: "job2"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}
		time.Sleep(1 * time.Millisecond)
		if err := testDB.InsertJob(db.Job{JobType: "job3"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}

		jobs, err := testDB.ListJobs(0)
		if err != nil {
			t.Fatalf("ListJobs(0) failed: %v", err)
		}
		if len(jobs) != baseline+3 {
			t.Fatalf("expected %d jobs, got %d", baseline+3, len(jobs))
		}
		// Verify descending order by creation time
		if jobs[0].JobType != "job3" {
			t.Errorf("expected first job to be 'job3', got '%s'", jobs[0].JobType)
		}
		if jobs[2].JobType != "job1" {
			t.Errorf("expected last job to be 'job1', got '%s'", jobs[2].JobType)
		}
	})

	t.Run("List with a limit", func(t *testing.T) {
		if err := testDB.InsertJob(db.Job{JobType: "limit_job1"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}
		if err := testDB.InsertJob(db.Job{JobType: "limit_job2"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}
		if err := testDB.InsertJob(db.Job{JobType: "limit_job3"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}

		jobs, err := testDB.ListJobs(2)
		if err != nil {
			t.Fatalf("ListJobs(2) failed: %v", err)
		}
		if len(jobs) != 2 {
			t.Errorf("expected 2 jobs with limit, got %d", len(jobs))
		}
	})
}

func (s QueueAdminSuite) TestDeleteJob(t *testing.T) {
	testDB := s.Db
	t.Run("Delete existing job", func(t *testing.T) {
		baselineJobs, _ := testDB.ListJobs(0)
		baseline := len(baselineJobs)
		if err := testDB.InsertJob(db.Job{JobType: "to_delete"}); err != nil {
			t.Fatalf("InsertJob failed: %v", err)
		}
		jobs, _ := testDB.ListJobs(0)
		jobID := jobs[0].ID

		err := testDB.DeleteJob(jobID)
		if err != nil {
			t.Fatalf("DeleteJob failed: %v", err)
		}

		// Verify the job is gone
		remainingJobs, _ := testDB.ListJobs(0)
		if len(remainingJobs) != baseline {
			t.Fatalf("expected %d jobs after deletion, got %d", baseline, len(remainingJobs))
		}
	})

	t.Run("Delete non-existent job", func(t *testing.T) {
		err := testDB.DeleteJob(99999)
		if err == nil {
			t.Fatal("expected an error when deleting a non-existent job, but got nil")
		}
		expectedErr := "no job found with ID 99999"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', got '%s'", expectedErr, err.Error())
		}
	})
}

func (s QueueAdminSuite) RunAll(t *testing.T) {
	s.TestListJobs(t)
	s.TestDeleteJob(t)
}
