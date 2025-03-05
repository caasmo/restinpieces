package app

import (
	"fmt"
	"net/http"
)

type jsonError struct {
	code int
	body []byte
}

var jsonHeader = []string{"application/json; charset=utf-8"}

// writeJSONError writes a precomputed JSON error response
func writeJSONError(w http.ResponseWriter, err jsonError) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(err.code)
	w.Write(err.body)
}

// writeJSONErrorf writes a formatted JSON error response
func writeJSONErrorf(w http.ResponseWriter, code int, format string, args ...interface{}) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(code)
	fmt.Fprintf(w, format, args...)
}
