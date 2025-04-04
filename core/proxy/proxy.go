package proxy

import (
	"net/http"
	"github.com/caasmo/restinpieces/core"
)

// represent the Proxy of an app handler
// mostly global conunters, metrics, and most important blocking, filtering 
// maybe can be part of app. app has a proxy, 
type Proxy struct {
	// TODO app http.Handler no Proxy needs all the services of app
	// app is also handler, the serverHttp method is to call its router
	app *core.App
}

// NewProxy creates a new Proxy instance with the given app
func NewProxy(app *core.App) *Proxy {
	return &Proxy{
		app: app,
	}
}

func (px *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get client IP from request using app's method
	ip := px.app.GetClientIP(r)

	// Block IP if it's not already blocked
	if !px.IsBlocked(ip) {
		if err := px.BlockIP(ip); err != nil {
			px.app.Logger().Error("failed to block IP", "ip", ip, "err", err)
		}
	}

	px.app.Router().ServeHTTP(w, r)
}

