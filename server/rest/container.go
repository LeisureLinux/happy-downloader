package rest

import (
	"github.com/go-chi/chi/v5"
	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	middlewares "github.com/LeisureLinux/happy-downloader/v4/server/middleware"
	"github.com/LeisureLinux/happy-downloader/v4/server/openid"
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
		if config.Instance().Authentication.RequireAuth {
			r.Use(middlewares.Authenticated)
		}
		if config.Instance().OpenId.UseOpenId {
			r.Use(openid.Middleware)
		}
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
	}
}
