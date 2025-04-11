package proxy

import (
	"net/http"

	//"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

// represent the Proxy of an app handler
// mostly global conunters, metrics, and most important blocking, filtering
// maybe can be part of app. app has a proxy,
type Proxy struct {
	// TODO app http.Handler no Proxy needs all the services of app
	// app is also handler, the serverHttp method is to call its router
	app       *core.App
	ipBlocker *BlockIp
}

// NewProxy creates a new Proxy instance with the given app and configures its features.
func NewProxy(app *core.App) *Proxy {
	px := &Proxy{
		app: app,
		// config is no longer stored directly on Proxy
		// mimetypeBlocker initialized below based on config
	}

	// Initialize the IP Blocker based on application configuration
	if app.Config().Proxy.BlockIp.Enabled {
		// Pass the application's cache and logger to the BlockIp implementation
		px.ipBlocker = NewBlockIp(app.Cache(), app.Logger())
	}

	return px
}

// UpdateByConfig method removed.

func (px *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	//px.app.Logger().Info("in proxy")
	// TODO
	handler := px.ipBlocker.Execute(px.app.Router())
	handler.ServeHTTP(w, r)

}
