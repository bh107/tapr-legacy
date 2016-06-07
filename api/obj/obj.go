package obj

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/bh107/tapr/server"
	"github.com/bh107/tapr/stream/policy"
	"github.com/gorilla/mux"
)

type Status struct {
	ID string
}

func internalServerError(rw http.ResponseWriter, err error) {
	log.Print(err)
	http.Error(rw, err.Error(), http.StatusInternalServerError)
}

func Store(srv *server.Server, rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	v := req.Header.Get("Timeout")
	timeout, err := time.ParseDuration(v)
	if err == nil {
		// The request has a timeout, so create a context that is
		// canceled automatically when the timeout expires.
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	defer cancel()

	pol, err := policy.Construct(req)
	if err != nil {
		panic(err)
	}

	ctx = policy.Wrap(ctx, pol)

	if archive, ok := vars["id"]; ok {
		if err := srv.Create(archive); err != nil {
			internalServerError(rw, err)
			return
		}

		if err := srv.Store(ctx, archive, req.Body); err != nil {
			internalServerError(rw, err)
			return
		}

		fmt.Fprint(rw, "OK")
		return
	}

	http.Error(rw, "Bad Request", http.StatusBadRequest)
}

func Retrieve(srv *server.Server, rw http.ResponseWriter, req *http.Request) {
	/*
		vars := mux.Vars(req)

			if archive, ok := vars["id"]; ok {
				if err := srv.Retrieve(rw, archive); err != nil {
					log.Print(err)
					http.Error(rw, err.Error(), http.StatusInternalServerError)
					return
				}

				return
			}
	*/

	http.Error(rw, "Not implemented", http.StatusNotImplemented)
}
