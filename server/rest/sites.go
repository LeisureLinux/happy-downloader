package rest

import (
	"encoding/json"
	"net/http"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/sites"
)

// Sites serves the catalogue of video sites.
//
// The interface draws its icons, its list of sites and the order they appear
// in from this. It used to have two lists of its own — one in the footer and
// one in the cookie dialog — which is how Facebook came to have an icon
// everywhere except in the dialog that is about cookies.
//
// The keys are what an icon is found by, so a site added here appears in the
// interface as soon as its mark exists on that side; a site whose mark does
// not exist yet is drawn as a globe rather than not at all.
func Sites(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]any{
		"sites": sites.All(),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
