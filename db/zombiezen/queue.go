package zombiezen	
import (
	"fmt"
	"github.com/caasmo/restinpieces/queue"
)

// CreateUser inserts a new user with RFC3339 formatted UTC timestamps
// InsertJob placeholder for zombiezen SQLite implementation
func (d *Db) Claim(limit int) ([]*queue.Job, error) {
	return nil, fmt.Errorf("Claim not implemented for zombiezen SQLite variant")
}

func (d *Db) GetJobs(limit int) ([]*queue.Job, error) {
	return nil, fmt.Errorf("GetJobs not implemented for zombiezen SQLite variant")
}

func (d *Db) InsertJob(job queue.Job) error {
	return fmt.Errorf("InsertJob not implemented for zombiezen SQLite variant")
}

func (d *Db) MarkCompleted(jobID int64) error {
	return fmt.Errorf("MarkCompleted not implemented for zombiezen SQLite variant")
}

func (d *Db) MarkFailed(jobID int64, errMsg string) error {
	return fmt.Errorf("MarkFailed not implemented for zombiezen SQLite variant")
}
