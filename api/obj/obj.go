package obj

import (
	"fmt"
	"log"
	"net/http"

	"github.com/bh107/tapr/server"
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

	if archive, ok := vars["id"]; ok {
		if err := srv.Create(archive); err != nil {
			internalServerError(rw, err)
			return
		}

		if err := srv.Store(archive, req.Body); err != nil {
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
