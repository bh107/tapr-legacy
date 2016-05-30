package obj

import (
	"fmt"
	"net/http"

	"github.com/bh107/tapr"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

type Status struct {
	ID string
}

func internalServerError(rw http.ResponseWriter, err error) {
	glog.Error(err)
	http.Error(rw, err.Error(), http.StatusInternalServerError)
}

func Store(srv *tapr.Server, rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	if archive, ok := vars["id"]; ok {
		if err := srv.Create(archive); err != nil {
			internalServerError(rw, err)
			return
		}

		if err := srv.Store(archive, req.Body, tapr.DefaultPolicy); err != nil {
			internalServerError(rw, err)
			return
		}

		fmt.Fprint(rw, "OK")
		return
	}

	http.Error(rw, "Bad Request", http.StatusBadRequest)
}

func Retrieve(srv *tapr.Server, rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	if archive, ok := vars["id"]; ok {
		if err := srv.Retrieve(rw, archive); err != nil {
			glog.Error(err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		return
	}

	http.Error(rw, "Not implemented", http.StatusNotImplemented)
}
