package api

import (
	"net/http"

	"golang.org/x/net/context"
)

// see https://joeshaw.org/net-context-and-http-handler/
// we need this stuff until net/http supports x/net/context.
type CtxHandler interface {
	CtxServeHTTP(context.Context, http.ResponseWriter, *http.Request)
}

type CtxHandlerFunc func(context.Context, http.ResponseWriter, *http.Request)

func (h CtxHandlerFunc) CtxServeHTTP(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	h(ctx, rw, req)
}

type CtxWrapper struct {
	ctx context.Context
	h   CtxHandler
}

func (h CtxWrapper) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.h.CtxServeHTTP(h.ctx, rw, req)
}
