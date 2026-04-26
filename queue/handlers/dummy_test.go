package handlers

import (
	"context"
	"testing"

	"github.com/caasmo/restinpieces/db"
)

func TestDummyHandler_Handle(t *testing.T) {
	handler := NewDummyHandler()
	job := db.Job{}
	err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}
}
