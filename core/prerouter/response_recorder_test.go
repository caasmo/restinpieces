package prerouter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/core"
)

// TestRecorderMiddleware verifies that the middleware correctly wraps the standard
// ResponseWriter in a core.ResponseRecorder and initializes its fields.
func TestRecorderMiddleware(t *testing.T) {
	// --- Setup ---
	mockApp := &core.App{}
	middleware := NewRecorder(mockApp)

	// The final handler is where we will run our assertions, because its purpose
	// is to inspect the ResponseWriter passed to it by the middleware.
	var finalHandler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Verify that the middleware passed a ResponseRecorder.
		recorder, ok := w.(*core.ResponseRecorder)
		if !ok {
			// Use t.Fatalf because the rest of the test is meaningless if this fails.
			t.Fatalf("Expected http.ResponseWriter to be a *core.ResponseRecorder, but it was not")
		}

		// 2. Verify the default status is correctly initialized.
		if recorder.Status != http.StatusOK {
			t.Errorf("Expected default status to be %d, but got %d", http.StatusOK, recorder.Status)
		}

		// 3. Verify the StartTime has been initialized.
		if recorder.StartTime.IsZero() {
			t.Error("Expected StartTime to be initialized, but it was a zero value")
		}
		// Check that the time is recent (within the last second).
		if time.Since(recorder.StartTime) > time.Second {
			t.Errorf("Expected StartTime to be recent, but it was %v", recorder.StartTime)
		}

		// 4. To prove the recorder works as a wrapper, write a different status code.
		w.WriteHeader(http.StatusAccepted)
	})

	handlerChain := middleware.Execute(finalHandler)
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	// --- Execution ---
	handlerChain.ServeHTTP(rr, req)

	// --- Verification ---
	// Verify that the WriteHeader call was passed through the wrapper to the
	// original ResponseRecorder.
	if rr.Code != http.StatusAccepted {
		t.Errorf("Expected final status code to be %d, but got %d", http.StatusAccepted, rr.Code)
	}
}
