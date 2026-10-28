package cookies

import (
	"os"
	"path/filepath"
	"testing"
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

	// The first refusal: counted, but the file stays. One refusal could be
	// a bad moment.
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

	// The second: the export is not working, so it goes.
	after = store.Failed()
	if after != nil {
		t.Error("the export survived the second refusal")
	}
	if store.Current() != nil {
		t.Error("the store still has an export after two refusals")
	}

	// The file itself is deleted, not merely ignored: a credential that no
	// longer works is not left lying around.
	if _, err := os.Stat(imported.Path); !os.IsNotExist(err) {
		t.Error("the imported file is still on disk")
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
