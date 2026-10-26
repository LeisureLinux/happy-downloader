package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

		"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1000,
	WriteBufferSize: 1000,
}

func webSocket(logger *ObservableLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, line := range logger.History() {
			if err := c.WriteJSON(string(line)); err != nil {
				return
			}
		}

		logs := logger.Observe(r.Context())

		for {
			select {
			case <-r.Context().Done():
				return
			case msg := <-logs:
				c.WriteJSON(msg)
			}
		}
	}
}

func sse(logger *ObservableLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// proxy_buffering off in the nginx block is not always enough: the
		// upstream header is what actually disables it, and without this the
		// stream arrives in bursts every few kilobytes instead of line by line.
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		// History first, so the panel opens with the lines that explain why it
		// was opened. Then the live stream continues from there.
		for _, line := range logger.History() {
			if _, err := fmt.Fprintf(w, "event: log\ndata: %s\n\n", mustJSON(line)); err != nil {
				return
			}
		}
		flusher.Flush()

		logs := logger.Observe(r.Context())

		for {
			select {
			case <-r.Context().Done():
				slog.Info("detaching from logger")
				return
			case msg, ok := <-logs:
				if !ok {
					http.Error(w, "closed logs channel", http.StatusInternalServerError)
					return
				}

				var b bytes.Buffer

				b.WriteString("event: log\n")
				b.WriteString("data: ")

				if err := json.NewEncoder(&b).Encode(msg); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				b.WriteRune('\n')
				b.WriteRune('\n')

				io.Copy(w, &b)

				flusher.Flush()
			}
		}
	}
}

// mustJSON encodes a history line for the SSE data field. The lines are plain
// log text, so this is only escaping.
func mustJSON(line []byte) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(string(line))
	// Encode appends a newline; the SSE frame supplies its own.
	return bytes.TrimRight(b.Bytes(), "\n")
}

func ApplyRouter(logger *ObservableLogger) func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/ws", webSocket(logger))
		r.Get("/sse", sse(logger))
	}
}
