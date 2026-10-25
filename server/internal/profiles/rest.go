package profiles

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewRestHandler exposes the profile table so the UI can add a site without
// touching the binary.
func NewRestHandler(s *Store) func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/", Get(s))
		r.Put("/", Replace(s))
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
