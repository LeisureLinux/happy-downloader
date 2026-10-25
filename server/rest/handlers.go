package rest

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
)

type Handler struct {
	service *Service
}

/*
	REST version of the JSON-RPC interface
*/

func (h *Handler) Exec() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		var req internal.DownloadRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		id, err := h.service.Exec(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (h *Handler) ExecPlaylist() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		var req internal.DownloadRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		err := h.service.ExecPlaylist(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode("ok"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (h *Handler) ExecLivestream() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		var req internal.DownloadRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		h.service.ExecLivestream(req)

		if err := json.NewEncoder(w).Encode("ok"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (h *Handler) Running() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		res, err := h.service.Running(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(res); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (h *Handler) CookiesStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		status, err := h.service.CookieStatus()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(status); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (h *Handler) AddTemplate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		req := new(internal.CustomTemplate)

		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Name == "" || req.Content == "" {
			http.Error(w, "Invalid template", http.StatusBadRequest)
			return
		}

		if err := h.service.SaveTemplate(r.Context(), req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode("ok"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (h *Handler) GetTemplates() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		templates, err := h.service.GetTemplates(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(templates)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (h *Handler) UpdateTemplate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		req := &internal.CustomTemplate{}

		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		res, err := h.service.UpdateTemplate(r.Context(), req)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(res); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (h *Handler) DeleteTemplate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		id := chi.URLParam(r, "id")

		if err := h.service.DeleteTemplate(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode("ok"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (h *Handler) GetVersion() http.HandlerFunc {
	type Response struct {
		RPCVersion   string `json:"rpcVersion"`
		RPCBuiltAt   string `json:"rpcBuiltAt"`
		YtdlpVersion string `json:"ytdlpVersion"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		rpcVersion, builtAt, ytdlpVersion, err := h.service.GetVersion(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		res := Response{
			RPCVersion:   rpcVersion,
			RPCBuiltAt:   builtAt,
			YtdlpVersion: ytdlpVersion,
		}

		if err := json.NewEncoder(w).Encode(res); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
