package api // import "a4.io/blobstash/pkg/stash/api"

import (
	"github.com/gorilla/mux"
	"net/http"

	"a4.io/blobstash/pkg/httputil"
	"a4.io/blobstash/pkg/stash"
)

type StashAPI struct {
	stash *stash.Stash
}

func New(s *stash.Stash) *StashAPI {
	return &StashAPI{s}
}

func (s *StashAPI) listHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		srw := httputil.NewSnappyResponseWriter(w, r)
		httputil.WriteJSON(srw, map[string]interface{}{})
		srw.Close()
	}
}

func (s *StashAPI) Register(r *mux.Router, basicAuth func(http.Handler) http.Handler) {
	r.Handle("/", basicAuth(http.HandlerFunc(s.listHandler())))
}
