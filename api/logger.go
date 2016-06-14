package api

import (
	"log"
	"net/http"
	"time"
)

func logger(h http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		h.ServeHTTP(w, r)

		log.Printf("api[%s]: %s %s    %s",
			name,
			r.Method,
			r.RequestURI,
			time.Since(start),
		)
	})
}
