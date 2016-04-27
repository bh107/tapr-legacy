package api

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/bh107/tapr"
	"github.com/bh107/tapr/api/cmd"
	"github.com/bh107/tapr/api/obj"
	"github.com/bh107/tapr/api/vol"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

type Route struct {
	name    string
	method  string
	pattern string
	handler CtxHandlerFunc
}

var routes = []Route{
	{"cmd/audit", "PATCH", "/cmd/audit/{library}", cmd.Audit},
	{"vol/list", "GET", "/vol/list/{library}", vol.List},
	{"obj/store", "PUT", "/obj/{id}", obj.Store},
	{"obj/retrieve", "GET", "/obj/{id}", obj.Retrieve},
}

// build routes from routes.go and wrap them with net/context
func NewRouter(srv *tapr.Server) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	for _, route := range routes {
		var handler http.Handler

		handler = &CtxWrapper{
			ctx: context.WithValue(context.Background(), "srv", srv),
			h:   route.handler,
		}

		handler = logger(handler, route.name)

		router.
			Methods(route.method).
			Path(route.pattern).
			Name(route.name).
			Handler(handler)
	}

	return router
}
func Start(srv *tapr.Server) error {
	router := NewRouter(srv)

	glog.Info("starting http server...")
	return http.ListenAndServe(":8080", router)
}
