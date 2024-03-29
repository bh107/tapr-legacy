package api

import (
	"log"
	"net/http"

	"github.com/bh107/tapr/api/cmd"
	"github.com/bh107/tapr/api/obj"
	"github.com/bh107/tapr/api/vol"
	"github.com/bh107/tapr/server"
	"github.com/gorilla/mux"
)

type Route struct {
	name    string
	method  string
	pattern string
	handler HandlerFunc
}

type Handler interface {
	ServeHTTP(*server.Server, http.ResponseWriter, *http.Request)
}

type HandlerFunc func(*server.Server, http.ResponseWriter, *http.Request)

func (h HandlerFunc) ServeHTTP(srv *server.Server, rw http.ResponseWriter, req *http.Request) {
	h(srv, rw, req)
}

type Wrapper struct {
	srv *server.Server
	h   Handler
}

func (w Wrapper) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	w.h.ServeHTTP(w.srv, rw, req)
}

var routes = []Route{
	{"cmd/audit", "PATCH", "/cmd/audit/{library}", cmd.Audit},
	{"vol/list", "GET", "/vol/list/{library}", vol.List},
	{"obj/store", "PUT", "/obj/{id}", obj.Store},
	{"obj/retrieve", "GET", "/obj/{id}", obj.Retrieve},
}

// build routes from routes.go and wrap them with net/context
func NewRouter(srv *server.Server) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	for _, route := range routes {
		var handler http.Handler

		handler = &Wrapper{
			srv: srv,
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
func Start(srv *server.Server) error {
	router := NewRouter(srv)

	log.Print("starting http2 server on https://localhost:8080")
	return http.ListenAndServeTLS(":8080", "cert.pem", "key.pem", router)
}
