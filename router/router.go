package router

import (
	"context"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

// Move to interface and this to new package for wrapper
// Route implmentations need to implement the interface
// Get maybe, but mos imporant NamedParams()
type Router struct {
	*httprouter.Router
}

func (r *Router) Get(path string, handler http.Handler) {
	r.Handler("GET", path, handler)
}

func New() *Router {
	return &Router{httprouter.New()}
}

// Implementations of iface router should define also struct implementing NamedParams
// TODO when own package, rename
type HttpRouterNamedParams struct{}

// Transform the httprouter context variable in touter independent Params
func (np *HttpRouterNamedParams) Get(ctx context.Context) Params {
	pms, _ := ctx.Value(httprouter.ParamsKey).(httprouter.Params)

	var params Params

	for _, v := range pms {
		p := Param{Key: v.Key, Value: v.Value}
		params = append(params, p)
	}

	return params
}

func NewHttpRouterNamedParams() *HttpRouterNamedParams {
	return &HttpRouterNamedParams{}
}
