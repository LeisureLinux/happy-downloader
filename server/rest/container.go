package rest

import (
	"github.com/go-chi/chi/v5"
)

func Container(args *ContainerArgs) *Handler {
	var (
		service = ProvideService(args)
		handler = ProvideHandler(service)
	)
	return handler
}

func ApplyRouter(args *ContainerArgs) func(chi.Router) {
	h := Container(args)

	return func(r chi.Router) {
		r.Post("/exec", h.Exec())
		r.Post("/execPlaylist", h.ExecPlaylist())
		r.Post("/execLivestream", h.ExecLivestream())
		r.Get("/running", h.Running())
		r.Get("/version", h.GetVersion())
		// Cookies are configured by path (paths.cookie_path) and picked up
		// automatically, so the UI only needs to report the current state.
		r.Get("/cookies/status", h.CookiesStatus())
		r.Post("/template", h.AddTemplate())
		r.Patch("/template", h.UpdateTemplate())
		r.Get("/template/all", h.GetTemplates())
		r.Delete("/template/{id}", h.DeleteTemplate())

		// Captions for a file that was already downloaded. Separate from the
		// download because YouTube rate-limits the caption endpoint far
		// harder, and a throttled caption request must not cost the video.
		captions := NewCaptionsHandler(args.SourceIndex)
		r.Post("/captions", captions.Download)
		r.Get("/captions", captions.Available)
		r.Get("/captions/status/{id}", captions.Status)
	}
}
