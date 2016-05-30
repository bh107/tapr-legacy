package cmd

import (
	"fmt"
	"net/http"

	"golang.org/x/net/context"

	"github.com/bh107/tapr"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

func Audit(srv *tapr.Server, rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if libname, ok := vars["library"]; ok {
		if _, err := srv.Audit(ctx, libname); err != nil {
			glog.Error(err)

			http.Error(rw, fmt.Sprintf("cmd/audit failed: %s", err),
				http.StatusInternalServerError,
			)

			return
		}

		fmt.Fprint(rw, "OK")
	}

	http.Error(rw, "Bad Request", http.StatusBadRequest)
}
