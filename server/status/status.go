package status

import (
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	"github.com/LeisureLinux/happy-downloader/v4/server/status/repository"
	"github.com/LeisureLinux/happy-downloader/v4/server/status/rest"
	"github.com/LeisureLinux/happy-downloader/v4/server/status/service"
	"github.com/go-chi/chi/v5"
)

func ApplyRouter(mdb *kv.Store) func(chi.Router) {
	var (
		r = repository.New(mdb)
		s = service.New(r, nil)
		h = rest.New(s)
	)

	return func(r chi.Router) {
		r.Get("/", h.Status())
	}
}
