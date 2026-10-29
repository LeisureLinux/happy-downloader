package kv

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	bolt "go.etcd.io/bbolt"
)

// Where a downloaded file came from.
//
// The download records are transient: a finished download leaves the queue, and
// the record goes with it. But a file on disk keeps needing to be connected to
// its source — to fetch its captions later, to show where it came from. So
// the connection is written down separately, keyed by the file's path, and it
// outlives the download.
//
// It also survives the file being deleted, because the index is cleaned up
// when a file is removed rather than when the download finishes.

var sourcesBucket = []byte("sources")

// SourceRecord is what is remembered about one downloaded file.
type SourceRecord struct {
	// Path is the file on disk.
	Path string `json:"path"`
	// URL is the page the file came from.
	URL string `json:"url"`
	// Site is the site the file came from, when the URL is not known.
	//
	// It exists for the files whose source cannot be recovered any more:
	// the container has no tags, and whatever recorded the download is
	// gone. The file's own name often says it — Instagram names its files
	// "Video by <user>" — and a site name that is known is better than the
	// globe that says nothing.
	//
	// The URL wins when there is one: the site is derived from it, and a
	// derived name cannot go stale the way a typed one can.
	Site string `json:"site,omitempty"`
	// Title is the title at the time of the download.
	Title string `json:"title"`
	// Captions are the caption languages the source offered.
	Captions []SourceCaption `json:"captions,omitempty"`
	// SavedAt is when the file was written.
	SavedAt int64 `json:"saved_at"`
}

// SourceCaption is one caption language a source offered.
type SourceCaption struct {
	Language  string `json:"language"`
	Name      string `json:"name,omitempty"`
	Automatic bool   `json:"automatic,omitempty"`
}

// SourceIndex remembers where the downloaded files came from.
type SourceIndex struct {
	db *bolt.DB

	mu  sync.RWMutex
	mem map[string]SourceRecord
}

// NewSourceIndex creates the index over an open database.
func NewSourceIndex(db *bolt.DB) (*SourceIndex, error) {
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(sourcesBucket)
		return err
	}); err != nil {
		return nil, err
	}

	idx := &SourceIndex{db: db, mem: map[string]SourceRecord{}}

	// Load what is already there, so a restart does not lose the index.
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(sourcesBucket)
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var rec SourceRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return nil
			}
			idx.mem[string(k)] = rec
			return nil
		})
	}); err != nil {
		return nil, err
	}

	return idx, nil
}

// Remember records where a file came from.
func (s *SourceIndex) Remember(rec SourceRecord) {
	if rec.Path == "" || rec.URL == "" {
		return
	}

	s.mu.Lock()
	s.mem[rec.Path] = rec
	s.mu.Unlock()

	if err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(sourcesBucket)
		if err != nil {
			return err
		}

		encoded, err := json.Marshal(rec)
		if err != nil {
			return err
		}

		return b.Put([]byte(rec.Path), encoded)
	}); err != nil {
		slog.Error("could not write the source record", slog.String("path", rec.Path), slog.Any("err", err))
	}
}

// LookupByName finds the source of a file by its name.
//
// The index is keyed by the full path, so a file that was moved into a folder
// leaves its record naming a path that is no longer there. The name is the
// same, though, and the name is what the record was written with.
//
// It is a fallback: the path is tried first, and only when that fails is the
// name looked for. Two files with the same name in different folders would
// be ambiguous, and the one whose record is still in the index is the one
// that is found — which is the one whose path is right anyway.
func (s *SourceIndex) LookupByName(path string) (SourceRecord, bool) {
	name := filepath.Base(filepath.Clean(path))
	if name == "" {
		return SourceRecord{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var found SourceRecord
	var foundAt string

	for recordedPath, rec := range s.mem {
		if filepath.Base(recordedPath) != name {
			continue
		}

		// The file still there wins: its record is the one that is right.
		if _, err := os.Stat(recordedPath); err == nil {
			return rec, true
		}

		if foundAt == "" {
			found = rec
			foundAt = recordedPath
		}
	}

	if foundAt == "" {
		return SourceRecord{}, false
	}

	return found, true
}

// Lookup finds the source of a file.
func (s *SourceIndex) Lookup(path string) (SourceRecord, bool) {
	clean := filepath.Clean(path)

	s.mu.RLock()
	rec, ok := s.mem[clean]
	s.mu.RUnlock()

	if ok {
		return rec, true
	}

	// Not in memory: it may have been written by an earlier run.
	var found SourceRecord

	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(sourcesBucket)
		if b == nil {
			return nil
		}

		v := b.Get([]byte(clean))
		if v == nil {
			return nil
		}

		return json.Unmarshal(v, &found)
	}); err != nil {
		return SourceRecord{}, false
	}

	if found.Path == "" {
		return SourceRecord{}, false
	}

	return found, true
}

// Move records that a file has moved.
//
// The index is keyed by the file's path, so a file that moves leaves its
// record behind — and the record is what says where the file came from. A
// file moved into a folder was showing no source at all, because the path in
// the index no longer named anything on disk.
//
// The record is moved rather than copied: there is one file, and it has one
// source.
func (s *SourceIndex) Move(from, to string) {
	fromClean := filepath.Clean(from)
	toClean := filepath.Clean(to)

	s.mu.Lock()
	rec, ok := s.mem[fromClean]
	if ok {
		rec.Path = toClean
		s.mem[toClean] = rec
		delete(s.mem, fromClean)
	}
	s.mu.Unlock()

	if err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(sourcesBucket)
		if err != nil {
			return err
		}

		encoded, err := json.Marshal(rec)
		if err != nil {
			return err
		}

		if err := b.Put([]byte(toClean), encoded); err != nil {
			return err
		}

		return b.Delete([]byte(fromClean))
	}); err != nil {
		slog.Error("could not move the source record",
			slog.String("from", fromClean),
			slog.String("to", toClean),
			slog.Any("err", err))
	}
}

// Forget removes a file's record.
func (s *SourceIndex) Forget(path string) {
	clean := filepath.Clean(path)

	s.mu.Lock()
	delete(s.mem, clean)
	s.mu.Unlock()

	if err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(sourcesBucket)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(clean))
	}); err != nil {
		slog.Error("could not remove the source record", slog.String("path", clean), slog.Any("err", err))
	}
}

// Prune drops the records of files that are no longer there.
//
// The index is meant to outlive a download, but not to grow without bound: a
// file that was deleted by hand should not keep its entry forever.
func (s *SourceIndex) Prune() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for path := range s.mem {
		if _, err := os.Stat(path); err != nil {
			delete(s.mem, path)

			if err := s.db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket(sourcesBucket)
				if b == nil {
					return nil
				}
				return b.Delete([]byte(path))
			}); err != nil {
				slog.Error("could not prune the source record", slog.String("path", path), slog.Any("err", err))
			}
		}
	}
}
