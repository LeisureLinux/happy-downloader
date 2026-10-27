package kv

import (
	"os"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func newTestIndex(t *testing.T) *SourceIndex {
	t.Helper()

	db, err := bolt.Open(filepath.Join(t.TempDir(), "test.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { db.Close() })

	idx, err := NewSourceIndex(db)
	if err != nil {
		t.Fatal(err)
	}

	return idx
}

func TestSourceIndexRoundTrip(t *testing.T) {
	idx := newTestIndex(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "Video.mp4")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	idx.Remember(SourceRecord{
		Path:  path,
		URL:   "https://youtu.be/abc",
		Title: "Video",
		Captions: []SourceCaption{
			{Language: "en"},
			{Language: "zh-Hans", Automatic: true},
		},
	})

	found, ok := idx.Lookup(path)
	if !ok {
		t.Fatal("the record was not found")
	}

	if found.URL != "https://youtu.be/abc" {
		t.Errorf("URL = %q", found.URL)
	}

	if len(found.Captions) != 2 {
		t.Errorf("captions = %+v", found.Captions)
	}
}

// The index is meant to outlive a download, so it is written to the database
// and read back after a restart.
func TestSourceIndexSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := bolt.Open(dbPath, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}

	idx, err := NewSourceIndex(db)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "Video.mp4")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	idx.Remember(SourceRecord{Path: path, URL: "https://youtu.be/abc"})

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	// A new process, the same database.
	db2, err := bolt.Open(dbPath, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}

	defer db2.Close()

	idx2, err := NewSourceIndex(db2)
	if err != nil {
		t.Fatal(err)
	}

	found, ok := idx2.Lookup(path)
	if !ok {
		t.Fatal("the record did not survive the restart")
	}

	if found.URL != "https://youtu.be/abc" {
		t.Errorf("URL = %q", found.URL)
	}
}

func TestSourceIndexForget(t *testing.T) {
	idx := newTestIndex(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "Video.mp4")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	idx.Remember(SourceRecord{Path: path, URL: "https://youtu.be/abc"})

	if _, ok := idx.Lookup(path); !ok {
		t.Fatal("the record was not found")
	}

	idx.Forget(path)

	if _, ok := idx.Lookup(path); ok {
		t.Error("the record survived being forgotten")
	}
}

// A file that was deleted by hand should not keep its entry forever.
func TestSourceIndexPrune(t *testing.T) {
	idx := newTestIndex(t)

	dir := t.TempDir()
	present := filepath.Join(dir, "Here.mp4")
	gone := filepath.Join(dir, "Gone.mp4")

	for _, p := range []string{present, gone} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	idx.Remember(SourceRecord{Path: present, URL: "https://youtu.be/a"})
	idx.Remember(SourceRecord{Path: gone, URL: "https://youtu.be/b"})

	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}

	idx.Prune()

	if _, ok := idx.Lookup(present); !ok {
		t.Error("the record of a file that is still there was pruned")
	}

	if _, ok := idx.Lookup(gone); ok {
		t.Error("the record of a deleted file was not pruned")
	}
}
