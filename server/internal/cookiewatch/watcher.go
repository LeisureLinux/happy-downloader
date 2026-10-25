// Package cookiewatch keeps the Netscape cookies.txt yt-dlp reads in sync with
// a JSON cookie export that the operator drops onto the host.
//
// The intended workflow is a headless one: the browser lives on another
// machine, the export is copied over (scp, a mounted share, an editor over
// SSH), and nothing else has to happen. There is no browser on this host to
// re-export from, so the file arriving is the only signal available.
//
// Two mechanisms cooperate, because neither is sufficient alone:
//
//   - An inotify watch reacts immediately to a write, rename or atomic
//     replace of the JSON file. Editors that save via a temp file and rename
//     are handled by watching the parent directory, not just the file.
//   - A modification-time check runs before each download. It covers the cases
//     inotify misses: the watch could not be established, the directory was
//     recreated on a new filesystem, a network mount does not deliver events,
//     or the file changed between the watch starting and the process noticing.
//
// A failed conversion never destroys the previous good cookies.txt. A malformed
// export is logged and skipped, leaving the last working file in place, because
// the alternative is that a partially copied file during an scp silently
// breaks downloads that were working a minute earlier.
package cookiewatch

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/internal/cookies"
)

// Watcher converts SourcePath into TargetPath whenever the source changes.
type Watcher struct {
	// SourcePath is the JSON (or any recognised format) export to read.
	SourcePath string
	// TargetPath is the Netscape cookies.txt to write for yt-dlp.
	TargetPath string

	// lastSourceMod records the source mtime of the last successful
	// conversion, so the pre-download check can skip redundant work.
	lastSourceMod time.Time
	// lastSummary describes the current target file.
	lastSummary cookies.Summary
	lastFormat  cookies.Format

	mu sync.Mutex

	watcher *fsnotify.Watcher
	done    chan struct{}
	once    sync.Once
}

// global holds the watcher so the HTTP status endpoint can report on it
// without threading a handle through every layer.
var (
	globalMu sync.RWMutex
	global   *Watcher
)

// SetGlobal registers the active watcher.
func SetGlobal(w *Watcher) {
	globalMu.Lock()
	global = w
	globalMu.Unlock()
}

// Global returns the active watcher, or nil when none is configured.
func Global() *Watcher {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

// New creates a watcher. SourcePath may be empty, in which case Sync is a
// no-op and the operator has not configured a cookie export.
func New(sourcePath, targetPath string) *Watcher {
	return &Watcher{
		SourcePath: sourcePath,
		TargetPath: targetPath,
		done:       make(chan struct{}),
	}
}

// Enabled reports whether a source path was configured.
func (w *Watcher) Enabled() bool {
	return w != nil && w.SourcePath != ""
}

// Start performs an initial conversion and then watches for changes.
//
// Errors establishing the watch are logged but not returned: the pre-download
// modification-time check still works without it, so a host whose filesystem
// does not support inotify (some network mounts) degrades instead of failing.
func (w *Watcher) Start() error {
	if !w.Enabled() {
		return nil
	}

	if err := w.Sync(); err != nil {
		// A bad file at startup must not prevent the server from running; the
		// operator may still be copying it over.
		slog.Warn("initial cookie conversion failed", slog.String("err", err.Error()))
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("cookie watch unavailable, falling back to mtime checks",
			slog.String("err", err.Error()))
		return nil
	}
	w.watcher = fw

	// Watch the parent directory as well as the file: an editor saving by
	// writing a temp file and renaming it replaces the inode, which a watch on
	// the old file would stop seeing.
	dir := filepath.Dir(w.SourcePath)
	if err := fw.Add(dir); err != nil {
		slog.Warn("cannot watch cookie directory, falling back to mtime checks",
			slog.String("dir", dir), slog.String("err", err.Error()))
		fw.Close()
		w.watcher = nil
		return nil
	}

	go w.loop()

	slog.Info("cookie watch started",
		slog.String("source", w.SourcePath),
		slog.String("target", w.TargetPath),
	)

	return nil
}

func (w *Watcher) loop() {
	// Debounce: a write often arrives as several events, and an scp produces
	// a burst. Coalescing them avoids converting a half-written file.
	var timer *time.Timer
	debounce := 400 * time.Millisecond

	schedule := func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(debounce, func() {
			select {
			case <-w.done:
				return
			default:
			}
			if err := w.Sync(); err != nil {
				slog.Warn("cookie conversion failed",
					slog.String("source", w.SourcePath),
					slog.String("err", err.Error()))
			}
		})
	}

	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			// Only react to the configured source. Watching the directory
			// means unrelated files (and the target itself) also produce
			// events.
			if filepath.Clean(event.Name) != filepath.Clean(w.SourcePath) {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Chmod) != 0 {
				slog.Info("cookie source changed",
					slog.String("path", event.Name),
					slog.String("op", event.Op.String()))
				schedule()
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("cookie watch error", slog.String("err", err.Error()))
		}
	}
}

// Stop releases the watch.
func (w *Watcher) Stop() {
	if w == nil {
		return
	}
	w.once.Do(func() {
		close(w.done)
		if w.watcher != nil {
			w.watcher.Close()
		}
	})
}

// Sync converts the source file if it is newer than the last conversion.
//
// This is the pre-download safety net: it costs one stat, and it covers every
// case where the watch is absent or missed an event.
func (w *Watcher) Sync() error {
	if !w.Enabled() {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	info, err := os.Stat(w.SourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			// The operator removed the export. Leave the existing good
			// cookies.txt alone; deleting it would break working downloads.
			return nil
		}
		return err
	}

	if !w.lastSourceMod.IsZero() && !info.ModTime().After(w.lastSourceMod) {
		return nil
	}

	return w.convertLocked(info)
}

// Force re-converts regardless of modification time.
func (w *Watcher) Force() error {
	if !w.Enabled() {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	info, err := os.Stat(w.SourcePath)
	if err != nil {
		return err
	}

	return w.convertLocked(info)
}

func (w *Watcher) convertLocked(info os.FileInfo) error {
	raw, err := os.ReadFile(w.SourcePath)
	if err != nil {
		return err
	}

	converted, format, err := cookies.Convert(raw)
	if err != nil {
		// Do not touch the target: the previous file may still be valid, and
		// a failed conversion is usually a partially written source.
		return fmt.Errorf("convert %s: %w", w.SourcePath, err)
	}

	summary := cookies.Summarize(converted)

	if err := os.MkdirAll(filepath.Dir(w.TargetPath), 0o755); err != nil {
		return err
	}

	// Write via a temp file and rename so yt-dlp never reads a partial file.
	tmp := w.TargetPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(converted), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, w.TargetPath); err != nil {
		os.Remove(tmp)
		return err
	}

	w.lastSourceMod = info.ModTime()
	w.lastSummary = summary
	w.lastFormat = format

	slog.Info("cookie file converted",
		slog.String("source", w.SourcePath),
		slog.String("target", w.TargetPath),
		slog.String("format", string(format)),
		slog.Int("count", summary.Total),
		slog.Int("httpOnly", summary.HttpOnly),
		slog.Int("session", summary.Session),
		slog.Int("expired", summary.Expired),
	)

	if missing := cookies.Required(summary.Names); len(missing) > 0 {
		slog.Warn("cookie file is missing YouTube login cookies; "+
			"requests may be treated as a guest",
			slog.Any("missing", missing))
	}

	return nil
}

// Status describes the current state for the HTTP status endpoint.
type Status struct {
	Exists     bool
	SourcePath string
	Format     string
	Count      int
	HttpOnly   int
	Session    int
	Expired    int
	Missing    []string
	ModifiedAt string
}

// Status reports what the target currently holds, converting first if the
// source is newer. Cookie values are never included.
func (w *Watcher) Status() Status {
	if !w.Enabled() {
		return Status{}
	}

	// Bring it up to date so the answer reflects the file on disk.
	_ = w.Sync()

	w.mu.Lock()
	defer w.mu.Unlock()

	st := Status{SourcePath: w.SourcePath}

	info, err := os.Stat(w.TargetPath)
	if err != nil {
		return st
	}
	st.Exists = true
	st.ModifiedAt = info.ModTime().Format(time.RFC3339)

	content, err := os.ReadFile(w.TargetPath)
	if err != nil {
		return st
	}

	s := cookies.Summarize(string(content))
	st.Format = string(w.lastFormat)
	if st.Format == "" {
		st.Format = string(cookies.Detect(content))
	}
	st.Count = s.Total
	st.HttpOnly = s.HttpOnly
	st.Session = s.Session
	st.Expired = s.Expired
	st.Missing = cookies.Required(s.Names)

	return st
}
