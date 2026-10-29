package logging

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloadlog"
)

// The download log, as the interface reads it.
//
// It is a file rather than a stream: a line is written once per finished
// download, so there is nothing to follow. Reading the tail of the file on
// demand is the whole of it, and it cannot break anything if the panel is
// closed while a download finishes.

// howManyLines is the default and the most that will be returned.
const (
	defaultDownloadLines = 300
	maxDownloadLines     = 2000
)

// downloads serves the tail of the download log.
func downloads(w http.ResponseWriter, r *http.Request) {
	limit := defaultDownloadLines
	if raw := r.URL.Query().Get("lines"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= maxDownloadLines {
			limit = n
		}
	}

	path := downloadlog.Path()

	w.Header().Set("Content-Type", "application/json")

	if path == "" {
		json.NewEncoder(w).Encode(map[string]any{"lines": []string{}, "path": ""})
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		// A log that has not been written yet is not an error: nothing has
		// finished, which is exactly what the panel should say.
		json.NewEncoder(w).Encode(map[string]any{"lines": []string{}, "path": path})
		return
	}

	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}

	json.NewEncoder(w).Encode(map[string]any{
		"lines": lines,
		"path":  path,
	})
}
