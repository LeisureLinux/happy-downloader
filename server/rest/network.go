package rest

import (
	"encoding/json"
	"net/http"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// The settings that belong to the installation rather than to a site.
//
// The PO Token provider and the proxy were per-site fields in the profile
// table, which meant the same value was typed into every row of it. They are
// one setting each now, and they are written back to the configuration file
// so that they survive a restart.
//
// NetworkConfig is what the interface reads and writes.
type NetworkConfig struct {
	// PotBaseURL is the bgutil PO Token provider, e.g.
	// http://127.0.0.1:4416. It is only used for YouTube: the challenge it
	// fetches is one only YouTube's extractor understands.
	PotBaseURL string `json:"potBaseUrl"`
	// Proxy is the proxy for the sites that need one. Which sites those
	// are is worked out when a download is built.
	Proxy string `json:"proxy"`
	// DownloadPath is where the finished files go.
	//
	// It belongs here rather than in the profile table because it is a
	// property of the installation: a file is downloaded once, wherever the
	// window happened to be open.
	DownloadPath string `json:"downloadPath"`
	// DefaultDownloadPath is what the application would use if the setting
	// were blank, so that the form can offer to go back to it.
	DefaultDownloadPath string `json:"defaultDownloadPath"`
}

// GetNetworkConfig reports the settings.
func (h *Handler) GetNetworkConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		conf := config.Instance()

		if err := json.NewEncoder(w).Encode(NetworkConfig{
			PotBaseURL:          conf.YtDlp.PotBaseURL,
			Proxy:               conf.YtDlp.Proxy,
			DownloadPath:        conf.Paths.DownloadPath,
			DefaultDownloadPath: config.DefaultDownloadPath(),
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// SetNetworkConfig writes the settings back to the configuration file.
func (h *Handler) SetNetworkConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req NetworkConfig

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := config.Update(config.Editable{
			PotBaseURL:   req.PotBaseURL,
			Proxy:        req.Proxy,
			DownloadPath: req.DownloadPath,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode("ok"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
