package core

import (
	"log"
	"net/http"
)

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

