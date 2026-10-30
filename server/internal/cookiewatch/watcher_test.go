package cookiewatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/cookies"
)

const jsonExport = `[
  {"domain":".youtube.com","expires":1823905798,"httpOnly":true,
   "name":"LOGIN_INFO","path":"/","secure":true,"value":"TESTVALUE-NOT-A-REAL-TOKEN"},
  {"domain":".youtube.com","expires":1823905798,"httpOnly":true,
   "name":"HSID","path":"/","secure":false,"value":"AbCdEf"},
  {"domain":".youtube.com","expires":1823905798,"httpOnly":true,
   "name":"SID","path":"/","secure":false,"value":"g.a000yyy"},
  {"domain":".youtube.com","expires":1823905798,"httpOnly":true,
   "name":"__Secure-3PSID","path":"/","secure":true,"value":"g.a000xxx"}
]`

func setup(t *testing.T) (*Watcher, string, string) {
	t.Helper()

	dir := t.TempDir()
	src := filepath.Join(dir, "xxx-cookie.json")
	dst := filepath.Join(dir, "state", "cookies.txt")

	if err := os.WriteFile(src, []byte(jsonExport), 0o600); err != nil {
		t.Fatalf("setup source: %v", err)
	}

	return New(src, dst), src, dst
}

// The core contract: dropping a JSON export on the host produces the Netscape
// file yt-dlp reads, with the httpOnly login cookies intact.
func TestSyncConvertsJSONToNetscape(t *testing.T) {
	w, _, dst := setup(t)

	if err := w.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("target not written: %v", err)
	}

	st := w.Status()
	if !st.Exists {
		t.Fatal("Status reports no file after conversion")
	}
	if st.Count != 4 {
		t.Errorf("Count = %d, want 4", st.Count)
	}
	if st.HttpOnly != 4 {
		t.Errorf("HttpOnly = %d, want 4", st.HttpOnly)
	}
	if len(st.Missing) != 0 {
		t.Errorf("login cookies reported missing: %v", st.Missing)
	}
	if st.Format != string(cookies.FormatJSON) {
		t.Errorf("Format = %q, want json", st.Format)
	}

	if len(got) == 0 {
		t.Error("target file is empty")
	}
}

// The target must be 0600: it holds a live session credential.
func TestSyncWritesRestrictedPermissions(t *testing.T) {
	w, _, dst := setup(t)

	if err := w.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want 600", perm)
	}
}

// Sync must be cheap when nothing changed, and must pick up a newer export.
func TestSyncHonoursModificationTime(t *testing.T) {
	w, src, dst := setup(t)

	if err := w.Sync(); err != nil {
		t.Fatalf("first Sync: %v", err)
	}

	first, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// Second call with an unchanged source must not rewrite the target.
	time.Sleep(20 * time.Millisecond)
	if err := w.Sync(); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	second, _ := os.Stat(dst)
	if !second.ModTime().Equal(first.ModTime()) {
		t.Error("target rewritten although the source did not change")
	}

	// Replace the export with a smaller one and advance its mtime.
	updated := `[{"domain":".youtube.com","expires":1823905798,"httpOnly":true,
	  "name":"PREF","path":"/","secure":false,"value":"tz=UTC"}]`
	if err := os.WriteFile(src, []byte(updated), 0o600); err != nil {
		t.Fatalf("rewrite source: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(src, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := w.Sync(); err != nil {
		t.Fatalf("third Sync: %v", err)
	}

	st := w.Status()
	if st.Count != 1 {
		t.Errorf("Count = %d, want 1 after the export changed", st.Count)
	}
}

// A malformed export must not destroy the last working cookies.txt: a partial
// file during an scp would otherwise break downloads that were fine a moment
// earlier.
func TestSyncKeepsPreviousFileOnBadExport(t *testing.T) {
	w, src, dst := setup(t)

	if err := w.Sync(); err != nil {
		t.Fatalf("first Sync: %v", err)
	}

	good, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}

	// Truncated JSON, as if the copy were still in progress.
	if err := os.WriteFile(src, []byte(`[{"domain":".youtube.com","name":"SID"`), 0o600); err != nil {
		t.Fatalf("corrupt source: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(src, future, future)

	if err := w.Sync(); err == nil {
		t.Error("expected an error for a truncated export")
	}

	after, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("target vanished: %v", err)
	}
	if string(after) != string(good) {
		t.Error("target was overwritten by a failed conversion")
	}
}

// A missing source leaves the existing target alone rather than failing.
func TestSyncWithMissingSourceKeepsTarget(t *testing.T) {
	w, src, dst := setup(t)

	if err := w.Sync(); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	before, _ := os.ReadFile(dst)

	os.Remove(src)

	if err := w.Sync(); err != nil {
		t.Errorf("Sync with a missing source returned %v, want nil", err)
	}

	after, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("target vanished: %v", err)
	}
	if string(after) != string(before) {
		t.Error("target changed when the source was removed")
	}
}

// A watcher with no configured source does nothing at all, so a host without
// cookies keeps working.
func TestDisabledWatcherIsInert(t *testing.T) {
	w := New("", filepath.Join(t.TempDir(), "cookies.txt"))

	if w.Enabled() {
		t.Error("Enabled() = true with an empty source")
	}
	if err := w.Sync(); err != nil {
		t.Errorf("Sync on a disabled watcher: %v", err)
	}
	if st := w.Status(); st.Exists {
		t.Error("Status reports a file on a disabled watcher")
	}
}

// The watch is the primary mechanism; this asserts it actually fires.
func TestWatchConvertsOnChange(t *testing.T) {
	w, src, dst := setup(t)

	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("initial conversion did not happen: %v", err)
	}

	// Write a new export the way an editor would: temp file then rename, which
	// replaces the inode.
	updated := `[{"domain":".youtube.com","expires":1823905798,"httpOnly":true,
	  "name":"SID","path":"/","secure":false,"value":"g.a000zzz"}]`
	tmp := src + ".new"
	if err := os.WriteFile(tmp, []byte(updated), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if err := os.Rename(tmp, src); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// The watcher debounces, so allow it time to settle.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(dst)
		if err == nil && cookies.Summarize(string(content)).Total == 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	content, _ := os.ReadFile(dst)
	t.Fatalf("watch did not convert the replacement; target has %d cookies",
		cookies.Summarize(string(content)).Total)
}

// A replaced export has to be converted even when its timestamp does not move.
//
// Sync compares the modification time with the last conversion, which is a
// reasonable check for the pre-download safety net — it costs one stat and it
// is called for every download. It is the wrong check after a watch event,
// because the event has already established that the file changed. Some
// filesystems only record whole seconds, and some editors write a file back
// with the timestamp it had, so the second opinion can disagree with the
// event and the conversion silently does not happen.
func TestWatchConvertsEvenWhenTheTimestampDoesNotMove(t *testing.T) {
	w, src, dst := setup(t)

	if err := w.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Stop()

	before, err := os.Stat(src)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}

	// One cookie instead of four, written in place and then given the
	// timestamp the file had before: this is what a coarse filesystem or a
	// careful editor produces.
	updated := `[{"domain":".youtube.com","expires":1823905798,"httpOnly":true,
	  "name":"SID","path":"/","secure":false,"value":"g.a000zzz"}]`

	if err := os.WriteFile(src, []byte(updated), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := os.Chtimes(src, before.ModTime(), before.ModTime()); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	// Sync, the safety net, is expected to decline: nothing looks newer.
	// The watch must not.
	if err := w.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if err := w.Force(); err != nil {
		t.Fatalf("Force: %v", err)
	}

	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}

	if got := cookies.Summarize(string(content)).Total; got != 1 {
		t.Fatalf("the target still has %d cookies; the replacement was not converted", got)
	}
}
