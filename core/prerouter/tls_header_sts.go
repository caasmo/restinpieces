package prerouter

import (
	"net/http"

	"github.com/caasmo/restinpieces/core"
)

type TLSHeaderSTS struct{}

func NewTLSHeaderSTS() *TLSHeaderSTS {
	return &TLSHeaderSTS{}
}

func (m *TLSHeaderSTS) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil {
			core.SetHeaders(w, core.HeadersTls)
		}
		next.ServeHTTP(w, r)
	})
}
