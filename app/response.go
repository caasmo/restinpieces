package app

import (
	"fmt"
	"net/http"
)

// writeError handles all error responses with precomputed values
func writeError(w http.ResponseWriter, e struct{code int; body []byte}) {
	h := w.Header()
	h["Content-Type"] = jsonHeader
	w.WriteHeader(e.code)
	w.Write(e.body)
}

// writeDynamicError handles errors with variable messages
func writeDynamicError(w http.ResponseWriter, code int, format string, args ...any) {
	h := w.Header()
	h["Content-Type"] = jsonHeader
	w.WriteHeader(code)
	fmt.Fprintf(w, format, args...)
}
