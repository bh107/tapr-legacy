package cmd

import (
	"fmt"
	"log"
	"net/http"

	"golang.org/x/net/context"

	"github.com/bh107/tapr/server"
	"github.com/gorilla/mux"
)

func Audit(srv *server.Server, rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if libname, ok := vars["library"]; ok {
		if _, err := srv.Audit(ctx, libname); err != nil {
			log.Print(err)

			http.Error(rw, fmt.Sprintf("cmd/audit failed: %s", err),
				http.StatusInternalServerError,
			)

			return
		}

		fmt.Fprint(rw, "OK")
	}

	http.Error(rw, "Bad Request", http.StatusBadRequest)
}
