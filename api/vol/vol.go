package vol

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/bh107/tapr/server"
	"github.com/gorilla/mux"
)

func List(srv *server.Server, rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	if libname, ok := vars["library"]; ok {

		vols, err := srv.Volumes(libname)
		if err != nil {
			log.Print(err)
			http.Error(rw, "volumes/list failed", http.StatusInternalServerError)

			return
		}

		js, err := json.Marshal(vols)
		if err != nil {
			log.Print(err)
			http.Error(rw, "volumes/list failed", http.StatusInternalServerError)

			return
		}

		rw.Header().Set("Content-Type", "applicaton/json")
		rw.Write(js)
	}

	http.Error(rw, "Bad Request", http.StatusBadRequest)
}
