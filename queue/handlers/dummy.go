package handlers

import (
	"context"

	"github.com/caasmo/restinpieces/db"
)

const JobTypeDummy = "job_type_dummy"

// PayloadDummy is used for timing equalization jobs.
// It contains a random ID to ensure that dummy jobs never hit
// unique constraints intended for real jobs.
type PayloadDummy struct {
	DummyID string `json:"dummy_id"`
}

// DummyHandler intentionally does nothing. It exists solely to successfully
// complete JobTypeDummy jobs that are inserted by auth endpoints to defeat
// timing attacks. By returning nil, the job is safely marked completed and purged.
type DummyHandler struct{}

func NewDummyHandler() *DummyHandler {
	return &DummyHandler{}
}

func (h *DummyHandler) Handle(ctx context.Context, job db.Job) error {
	return nil
}
