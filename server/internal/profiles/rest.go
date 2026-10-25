package profiles

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ArgvBuilder resolves the yt-dlp command line for a URL. It is injected
// rather than imported so this package stays free of the downloaders package,
// which imports this one for profile lookup — a direct import would be a cycle.
type ArgvBuilder func(url string) []string

// NewRestHandler exposes the profile table so the UI can add a site without
// touching the binary.
func NewRestHandler(s *Store, build ArgvBuilder) func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/", Get(s))
		r.Put("/", Replace(s))
		r.Get("/preview", Preview(s, build))
	}
}

// Preview returns the yt-dlp argv a URL would resolve to, without running it.
//
// The argument set is assembled from data (the profile table plus the global
// config), so the only way an operator can confirm what will actually be
// passed — and the only way to debug a failure that depends on it — is to ask
// for the resolved command line.
func Preview(s *Store, build ArgvBuilder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		raw := r.URL.Query().Get("url")
		if raw == "" {
			http.Error(w, "url query parameter is required", http.StatusBadRequest)
			return
		}

		profile, matched := ForURL(raw)

		name := ""
		if matched {
			name = profile.Name
		}

		resp := map[string]any{
			"url":     raw,
			"matched": matched,
			"profile": name,
			"argv":    build(raw),
		}

		_ = json.NewEncoder(w).Encode(resp)
	}
}

func Get(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(s.Table()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func Replace(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		var t table
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := s.Replace(t); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(s.Table()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
