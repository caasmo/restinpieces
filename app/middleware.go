package app

import (
	"log"
	"net/http"
	"time"
)

// SecurityHeadersMiddleware adds security headers to all responses
// TODO
func (a *App) SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("Cache-Control", "no-store")
		h.Set("Pragma", "no-cache")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// All middleware should conform to fn(next http.Handler) http.Handler
//
// Differentiate from the Handler by ussing suffix
func (a *App) Auth(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		authToken := r.Header.Get("Authorization")
		//user, err := map[string]interface{}{}, errors.New("test")
		//user := authToken
		// user, err := getUser(c.db, authToken)
		log.Println(authToken)

		// if return in middleware, no next, chain stopped
		//if err != nil {
		//    http.Error(w, http.StatusText(401), 401)
		//    return
		//}

		// TODO communication betwwen handlers
		//context.Set(r, "user", user)
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func (a *App) Logger(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		t1 := time.Now()
		next.ServeHTTP(w, r)
		t2 := time.Now()
		log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), t2.Sub(t1))
	}

	return http.HandlerFunc(fn)
}
