package httprouter

import (
	"context"
	"github.com/caasmo/restinpieces/router"
	jshttprouter "github.com/julienschmidt/httprouter"
	"net/http"
)

// Implementation of the router interface
type Router struct {
	rt *jshttprouter.Router
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.rt.ServeHTTP(w, req)
}

func (r *Router) Get(path string, handler http.Handler) {
	r.rt.Handler("GET", path, handler)
}

func New() router.Router {
	return &Router{rt: jshttprouter.New()}
}

// Implementation of the router/ParamGeter interface
type jsParams struct{}

func (js *jsParams) Get(ctx context.Context) router.Params {
	pms, _ := ctx.Value(jshttprouter.ParamsKey).(jshttprouter.Params)

	var params router.Params

	for _, v := range pms {
		p := router.Param{Key: v.Key, Value: v.Value}
		params = append(params, p)
	}

	return params
}

func NewParamGeter() router.ParamGeter {
	return &jsParams{}
}
