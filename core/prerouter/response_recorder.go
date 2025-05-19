package prerouter

import (
	"net/http"
	"time"
	"github.com/caasmo/restinpieces/core"
)

type Recorder struct {
	app *core.App
}

func NewRecorder(app *core.App) *Recorder {
	return &Recorder{
		app: app,
	}
}

// RecorderMiddleware initializes the shared recorder at the beginning of the chain
func (r *Recorder) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &core.ResponseRecorder{
			ResponseWriter: w,
			Status:        http.StatusOK, // Default to 200 OK
			StartTime:     time.Now(),
			//RequestID:     r.Header.Get("X-Request-ID"), // Optional
		}
		
		// Continue chain with our recorder
		next.ServeHTTP(recorder, r)
	})
}
