package prerouter

import (
	"net/http"

	"github.com/caasmo/restinpieces/core"
)

// TLSHeaderSTS is the middleware that adds the HSTS response headers from
// core.HeadersTls.
//
// HSTS (HTTP Strict Transport Security) tells the browser to use HTTPS on
// this site for a while, even when the visitor types an "http://" address.
// Browsers only obey it when the visitor's connection is HTTPS, so this
// middleware sends the headers only for requests that arrived over TLS. It
// asks App.ClientUsesTLS, which also understands a proxy header that reports
// the visitor's connection when a proxy terminates TLS and forwards plain
// HTTP to this server.
type TLSHeaderSTS struct {
	app *core.App
}

// NewTLSHeaderSTS creates the middleware for an app. The app supplies the
// configuration that names the proxy header.
func NewTLSHeaderSTS(app *core.App) *TLSHeaderSTS {
	return &TLSHeaderSTS{
		app: app,
	}
}

// Execute wraps the next handler, setting the HSTS headers first when the
// request arrived over TLS.
func (m *TLSHeaderSTS) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.app.ClientUsesTLS(r) {
			core.SetHeaders(w, core.HeadersTls)
		}
		next.ServeHTTP(w, r)
	})
}
