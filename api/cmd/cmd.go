package cmd

import (
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/kbj/tapr"

	"golang.org/x/net/context"
)

func Audit(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	srv := ctx.Value("srv").(*tapr.Server)
	vars := mux.Vars(req)

	if libname, ok := vars["library"]; ok {
		if _, err := srv.Audit(libname); err != nil {
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
