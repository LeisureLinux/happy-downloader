package rest

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/tools"
)

// Reading the session out of a browser instead of a file.
//
// A cookie export is a photograph: taken at one moment, kept until somebody
// remembers to take another. The browser's own database is the thing that
// changes, so reading it means a download next week still has next week's
// session.
//
// It does not always work, and the failure is not obvious. Chromium browsers
// since version 127 encrypt their cookie database with a key bound to the
// browser process, which nothing outside the browser can decrypt — yt-dlp
// reports "failed to decrypt with DPAPI" and the maintainers have marked it as
// not something they will fix. Firefox has no such protection and works.
//
// So the answer is checked rather than trusted: yt-dlp is asked to read the
// cookies, and what it says is reported back, including the count.

// CookieBrowserConfig is what the interface reads and writes.
type CookieBrowserConfig struct {
	// FromBrowser is the yt-dlp name of the browser, or empty for a file.
	FromBrowser string `json:"fromBrowser"`
	// Choices are the values that can be passed. They are yt-dlp's names,
	// because inventing our own would mean translating them back.
	Choices []string `json:"choices"`
}

// CookiesFromBrowserChoices is what the interface offers.
//
// Firefox comes first because it is the one that works: it stores cookies in a
// plain SQLite database, while Chromium browsers lock theirs while running and,
// since 127, encrypt it with a key tied to the browser process.
var CookiesFromBrowserChoices = []string{
	"firefox", "chrome", "chromium", "edge", "brave", "opera", "vivaldi",
}

type cookieBrowserRequest struct {
	FromBrowser string `json:"fromBrowser"`
}

// CookieProbeResult is what asking yt-dlp to read the cookies produced.
type CookieProbeResult struct {
	// OK is true when cookies were read.
	OK bool `json:"ok"`
	// Cookies is how many were read.
	Cookies int `json:"cookies"`
	// Problem is why not, in the words of whoever has to act on it.
	Problem string `json:"problem,omitempty"`
	// Detail is yt-dlp's own output, for the log panel.
	Detail string `json:"detail,omitempty"`
}

// GetCookieBrowser reports the setting and what can be chosen.
func (h *Handler) GetCookieBrowser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(CookieBrowserConfig{
			FromBrowser: config.Instance().Cookies.FromBrowser,
			Choices:     CookiesFromBrowserChoices,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// SetCookieBrowser saves the setting and reports whether it works.
//
// The check is part of saving rather than something the user has to know to
// ask for: a setting that was accepted and does not work turns into a download
// that fails for no stated reason.
func (h *Handler) SetCookieBrowser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req cookieBrowserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		browser := strings.TrimSpace(req.FromBrowser)

		if err := config.Update(config.Editable{
			PotBaseURL:  config.Instance().YtDlp.PotBaseURL,
			Proxy:       config.Instance().YtDlp.Proxy,
			FromBrowser: browser,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if browser == "" {
			// Nothing to check: a file is used instead.
			_ = json.NewEncoder(w).Encode(CookieProbeResult{OK: true})
			return
		}

		result := ProbeCookiesFromBrowser(r.Context(), browser)

		if err := json.NewEncoder(w).Encode(result); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// ProbeCookies asks yt-dlp whether the configured browser can be read.
func (h *Handler) ProbeCookies() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		browser := config.Instance().Cookies.FromBrowser
		if browser == "" {
			_ = json.NewEncoder(w).Encode(CookieProbeResult{OK: true})
			return
		}

		if err := json.NewEncoder(w).Encode(ProbeCookiesFromBrowser(r.Context(), browser)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// probeTimeout bounds the probe. Reading the database is local and quick; the
// network step it may reach for is not what is being measured.
var probeTimeout = 25 * time.Second

// probeURL is what the probe asks yt-dlp to look at.
//
// Any address would do — the cookies are read before the request is made —
// but this one is one yt-dlp's own test suite uses, so it is stable and it
// exists.
const probeURL = "https://www.youtube.com/watch?v=BaW_jenozKc"

// The lines worth reading out of yt-dlp's output.
var (
	// "Extracted 42 cookies from firefox" — the summary.
	cookiesFoundPattern = regexp.MustCompile(`Extracted\s+(\d+)\s+cookies`)
	// "[Cookies] Loading cookie 12/30" — the progress, which is where the
	// count appears when the run does not reach the summary.
	cookieProgressPattern = regexp.MustCompile(`Loading cookie\s+(\d+)\s*/\s*(\d+)`)
	// Any line yt-dlp marks as an error.
	errorLinePattern = regexp.MustCompile(`(?m)^ERROR:`)
)

// ProbeCookiesFromBrowser asks yt-dlp to read the cookies of one browser.
func ProbeCookiesFromBrowser(ctx context.Context, browser string) CookieProbeResult {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	args := []string{
		"--ignore-config",
		"--cookies-from-browser", browser,
		// Nothing is downloaded; the cookies are read before any of this
		// matters, and a network failure afterwards is not what is being
		// reported.
		"--simulate",
		"--no-warnings",
		probeURL,
	}

	cmd := exec.CommandContext(ctx, tools.YtDlp(), args...)
	tools.Quiet(cmd)

	out, err := cmd.CombinedOutput()
	text := string(out)

	result := classifyProbeOutput(text)
	result.Detail = lastLines(text, 20)

	if result.Problem != "" {
		slog.Warn("cookies could not be read from the browser",
			slog.String("browser", browser),
			slog.String("problem", result.Problem))
	}

	if err != nil && !result.OK && result.Problem == "" {
		result.Problem = "unreadable"
	}

	return result
}

// classifyProbeOutput reads yt-dlp's report of what it could read.
//
// It is separate from running the command so that the decisions can be checked
// against real output without a browser on the machine.
func classifyProbeOutput(text string) CookieProbeResult {
	var result CookieProbeResult

	// "failed to decrypt" is the App-Bound Encryption answer, and it deserves
	// its own sentence: nothing the user can do in this program will fix it.
	switch {
	case strings.Contains(text, "failed to decrypt"),
		strings.Contains(text, "Failed to decrypt"):
		result.Problem = "encrypted"

		return result

	case strings.Contains(text, "could not copy"),
		strings.Contains(text, "Permission denied"):
		result.Problem = "locked"

		return result
	}

	if m := cookiesFoundPattern.FindStringSubmatch(text); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			result.OK = true
			result.Cookies = n

			return result
		}
	}

	// No summary. The loading progress is the other place the answer appears,
	// and a non-zero total there means the database was read.
	if m := cookieProgressPattern.FindStringSubmatch(text); m != nil {
		if total, err := strconv.Atoi(m[2]); err == nil && total > 0 {
			result.OK = true
			result.Cookies = total

			return result
		}
	}

	// An error with no explanation above it is still an answer: nothing could
	// be read.
	if errorLinePattern.MatchString(text) {
		result.Problem = "unreadable"
	}

	return result
}

// lastLines keeps the end of the output, which is where yt-dlp puts the
// reason.
func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return strings.Join(lines, "\n")
}
