package prerouter

import (
	"net/http"
	"time"
	"github.com/caasmo/restinpieces/core"
)

// ResponseRecorder is a comprehensive recorder that captures various HTTP response metrics
// TODO move from here, to core
type ResponseRecorder struct {
	http.ResponseWriter
	Status        int           // HTTP status code
	WroteHeader   bool          // Flag to track if headers were written
	BytesWritten  int64         // Total bytes written to response
	StartTime     time.Time     // When the request started
	RequestID     string        // Optional request ID for tracing
}

// Implement necessary methods to properly capture metrics

func (r *ResponseRecorder) WriteHeader(status int) {
	if !r.WroteHeader {
		r.Status = status
		r.WroteHeader = true
		r.ResponseWriter.WriteHeader(status)
	}
}

func (r *ResponseRecorder) Write(b []byte) (int, error) {
	// If headers not written yet, call WriteHeader with default status
	if !r.WroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	
	n, err := r.ResponseWriter.Write(b)
	r.BytesWritten += int64(n)
	return n, err
}

// Add helper methods that middlewares might need

// TODO
//func (r *ResponseRecorder) Duration() time.Duration {
//	return time.Since(r.StartTime)
//}

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
		recorder := &ResponseRecorder{
			ResponseWriter: w,
			Status:        http.StatusOK, // Default to 200 OK
			StartTime:     time.Now(),
			//RequestID:     r.Header.Get("X-Request-ID"), // Optional
		}
		
		// Continue chain with our recorder
		next.ServeHTTP(recorder, r)
	})
}
