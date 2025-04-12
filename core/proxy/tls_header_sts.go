package proxy

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
			core.SetHeaders(w, core.headersTls)
		}
		next.ServeHTTP(w, r)
	})
}
