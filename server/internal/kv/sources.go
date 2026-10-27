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
