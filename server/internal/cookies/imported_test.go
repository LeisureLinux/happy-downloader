package cookies

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A valid upload is accepted and becomes the export in use.
func TestImportAcceptsAValidExport(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	upload := `[
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"LOGIN_INFO","path":"/","secure":true,"value":"AF1Qiz"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"HSID","path":"/","secure":true,"value":"AbCdEf"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"SID","path":"/","secure":true,"value":"g.a000y"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"__Secure-3PSID","path":"/","secure":true,"value":"g.a000x"}
	]`

	imported, err := store.Import([]byte(upload))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if imported.Count != 4 {
		t.Errorf("count = %d, want 4", imported.Count)
	}

	if imported.Format != string(FormatJSON) {
		t.Errorf("format = %q, want json", imported.Format)
	}

	if store.Current() == nil {
		t.Error("the store has no current export after an import")
	}

	// The file has to be on disk, because that is what the downloader reads.
	if _, err := os.Stat(imported.Path); err != nil {
		t.Errorf("the imported file is not there: %v", err)
	}
}

// A guest session is refused at upload time.
//
// Storing it would only produce a download that fails later, with the reason
// a long way from the mistake.
func TestImportRefusesAGuestSession(t *testing.T) {
	store := NewStore(t.TempDir())

	guest := `[
	  {"domain":".youtube.com","name":"PREF","path":"/","session":true,"value":"tz=UTC"}
	]`

	if _, err := store.Import([]byte(guest)); err == nil {
		t.Error("a guest session was accepted")
	}

	if store.Current() != nil {
		t.Error("a refused upload became the export in use")
	}
}

// A file that cannot be recognised is refused.
func TestImportRefusesUnrecognisedContent(t *testing.T) {
	store := NewStore(t.TempDir())

	if _, err := store.Import([]byte("not a cookie file at all")); err == nil {
		t.Error("unrecognised content was accepted")
	}
}

// The export is removed after two refusals, and the automatic one takes over.
func TestFailedRemovesTheExportAfterTwoRefusals(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	upload := `[
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"LOGIN_INFO","path":"/","secure":true,"value":"AF1Qiz"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"HSID","path":"/","secure":true,"value":"AbCdEf"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"SID","path":"/","secure":true,"value":"g.a000y"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"__Secure-3PSID","path":"/","secure":true,"value":"g.a000x"}
	]`

	imported, err := store.Import([]byte(upload))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	// The first attempt: one refusal, and the file stays. One attempt could
	// be a bad moment.
	store.newAttempt()
	after := store.Failed()
	if after == nil {
		t.Fatal("the first refusal removed the export")
	}
	if after.Failures != 1 {
		t.Errorf("failures = %d, want 1", after.Failures)
	}
	if store.Current() == nil {
		t.Error("the export was forgotten after one refusal")
	}

	// The second refusal, seconds later: still the same failure that the
	// interface retried. The export stays.
	store.newAttempt()
	after = store.Failed()

	if after == nil {
		t.Fatal("the export was removed by a retry seconds after the first refusal")
	}
	if after.Failures != 2 {
		t.Errorf("failures = %d, want 2", after.Failures)
	}

	// The second refusal, long after the first: the export is not working.
	store.current.FirstRefusal = store.current.FirstRefusal.Add(-refusalSeparation - time.Minute)

	store.newAttempt()
	after = store.Failed()

	if after != nil {
		t.Error("the export survived the second, spaced-out refusal")
	}
	if store.Current() != nil {
		t.Error("the store still has an export after two spaced-out refusals")
	}

	// The file is kept for a while rather than deleted at once: a cookie
	// that has just expired is fixed by exporting it again, and the person
	// may well do that within the hour.
	if _, err := os.Stat(imported.Path); err != nil {
		t.Errorf("the imported file was deleted at once: %v", err)
	}
}

// One attempt counts once, however many warnings it produced.
//
// yt-dlp reports a rotated cookie once per request, and a download makes
// several requests. Counting each report would have removed the export after
// one attempt rather than two.
func TestOneAttemptCountsOnce(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	upload := `[
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"LOGIN_INFO","path":"/","secure":true,"value":"AF1Qiz"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"HSID","path":"/","secure":true,"value":"AbCdEf"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"SID","path":"/","secure":true,"value":"g.a000y"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"__Secure-3PSID","path":"/","secure":true,"value":"g.a000x"}
	]`

	if _, err := store.Import([]byte(upload)); err != nil {
		t.Fatalf("Import: %v", err)
	}

	// The first attempt: three requests, three warnings.
	store.newAttempt()
	for i := 0; i < 3; i++ {
		store.Failed()
	}

	cur := store.Current()
	if cur == nil {
		t.Fatal("the export was removed after one attempt")
	}
	if cur.Failures != 1 {
		t.Errorf("failures = %d, want 1 — one attempt is one failure however many warnings it produced", cur.Failures)
	}

	// The second attempt, long after the first: removed.
	store.current.FirstRefusal = store.current.FirstRefusal.Add(-refusalSeparation - time.Minute)

	store.newAttempt()
	store.Failed()

	if store.Current() != nil {
		t.Error("the export survived the second attempt")
	}
}

// The import survives a restart.
func TestRestoreFindsTheImportAgain(t *testing.T) {
	dir := t.TempDir()

	upload := `[
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"LOGIN_INFO","path":"/","secure":true,"value":"AF1Qiz"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"HSID","path":"/","secure":true,"value":"AbCdEf"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"SID","path":"/","secure":true,"value":"g.a000y"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"__Secure-3PSID","path":"/","secure":true,"value":"g.a000x"}
	]`

	store := NewStore(dir)
	if _, err := store.Import([]byte(upload)); err != nil {
		t.Fatalf("Import: %v", err)
	}

	// A new store over the same directory, as after a restart.
	restored := NewStore(dir)
	restored.Restore()

	cur := restored.Current()
	if cur == nil {
		t.Fatal("the import was not restored")
	}
	if cur.Count != 4 {
		t.Errorf("count = %d, want 4", cur.Count)
	}
}

// The path of the imported file is under the directory it was given, not
// somewhere else.
func TestImportStaysInsideItsDirectory(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	upload := `[
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"LOGIN_INFO","path":"/","secure":true,"value":"AF1Qiz"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"HSID","path":"/","secure":true,"value":"AbCdEf"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"SID","path":"/","secure":true,"value":"g.a000y"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"__Secure-3PSID","path":"/","secure":true,"value":"g.a000x"}
	]`

	imported, err := store.Import([]byte(upload))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	rel, err := filepath.Rel(dir, imported.Path)
	if err != nil || rel == ".." || filepath.IsAbs(rel) {
		t.Errorf("the imported file is at %q, outside %q", imported.Path, dir)
	}
}
