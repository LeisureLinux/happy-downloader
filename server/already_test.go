package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// A port held by this application is reported as such, with its version.
func TestRunningElsewhereFindsTheApplication(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/version" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"rpcVersion":"4.10.0","ytdlpPath":"/usr/bin/yt-dlp"}`))
	}))
	defer srv.Close()

	version, ok := RunningElsewhere(portOf(t, srv))

	if !ok {
		t.Fatal("the application was not recognised")
	}

	if version != "4.10.0" {
		t.Fatalf("version = %q, want 4.10.0", version)
	}
}

// Anything else that answers 200 on that port is not this application, and
// claiming otherwise would tell the user a program is already running when it
// is something else entirely.
func TestRunningElsewhereRejectsAnotherServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hello":"world"}`))
	}))
	defer srv.Close()

	if _, ok := RunningElsewhere(portOf(t, srv)); ok {
		t.Fatal("a server that is not this application was called ours")
	}
}

func TestRunningElsewhereRejectsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>hello</body></html>`))
	}))
	defer srv.Close()

	if _, ok := RunningElsewhere(portOf(t, srv)); ok {
		t.Fatal("an HTML page was called this application")
	}
}

// A port nobody is listening on is not running elsewhere.
func TestRunningElsewhereWithAFreePort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	if _, ok := RunningElsewhere(port); ok {
		t.Fatal("a free port was reported as running elsewhere")
	}
}

func portOf(t *testing.T, srv *httptest.Server) int {
	t.Helper()

	_, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("split %q: %v", srv.URL, err)
	}

	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("port %q: %v", port, err)
	}

	return n
}
