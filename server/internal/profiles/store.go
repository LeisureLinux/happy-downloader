package profiles

import (
	"encoding/json"
	"log/slog"
	"strings"
	"sync"

	bolt "go.etcd.io/bbolt"
)

const bucketName = "profiles"

// Store owns the profile table. Reads go through the package-level Table() so
// the hot path (every BuildArgs call) takes one RLock; writes go through the
// service.
type Store struct {
	db *bolt.DB

	mu    sync.RWMutex
	table table
}

var (
	globalMu sync.RWMutex
	global   *Store
)

// NewStore opens the bucket and seeds it with the defaults on first run.
//
// Seeding matters: an operator upgrading from a version without profiles
// should find YouTube, Bilibili and x.com already wired up, not an empty
// table to fill in before anything works.
func NewStore(db *bolt.DB, seed Profile) (*Store, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})
	if err != nil {
		return nil, err
	}

	s := &Store{db: db}

	raw := s.read()

	if len(raw) == 0 {
		t := defaultTable
		// Carry the seed (config-file cookie/POT/proxy) into the YouTube
		// entry and the fallback, so an existing deployment keeps working.
		if seed.CookiePath != "" || seed.PotBaseURL != "" || seed.Proxy != "" {
			for i := range t {
				// YouTube by name, plus the fallback: a URL no profile names
				// should still get the credentials the operator configured.
				if strings.Contains(t[i].HostSuffix, "youtube.com") || t[i].HostSuffix == "" {
					if seed.CookiePath != "" {
						t[i].CookiePath = seed.CookiePath
					}
					if seed.PotBaseURL != "" {
						t[i].PotBaseURL = seed.PotBaseURL
					}
					if seed.Proxy != "" {
						t[i].Proxy = seed.Proxy
					}
				}
			}
		}

		if err := s.write(t); err != nil {
			return nil, err
		}
		slog.Info("seeded default site profiles", slog.Int("count", len(t)))
	}

	s.table = s.read()

	globalMu.Lock()
	global = s
	globalMu.Unlock()

	return s, nil
}

// Table returns the live table.
func (s *Store) Table() []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.table.sortedCopy()
}

// Replace swaps the whole table. The UI edits profiles as a set, so a
// full replace is simpler and safer than per-row mutation.
func (s *Store) Replace(t table) error {
	// Normalise: exactly one fallback, and it stays last.
	var named, fallback table

	for _, p := range t {
		if strings.TrimSpace(p.HostSuffix) == "" {
			fallback = append(fallback, p)
			continue
		}
		named = append(named, p)
	}

	if len(fallback) == 0 {
		fallback = table{{Name: "Fallback"}}
	} else if len(fallback) > 1 {
		fallback = fallback[:1]
		slog.Warn("multiple fallback profiles supplied, keeping the first")
	}

	merged := append(named, fallback...)

	if err := s.write(merged); err != nil {
		return err
	}

	s.mu.Lock()
	s.table = merged
	s.mu.Unlock()

	slog.Info("site profiles replaced", slog.Int("count", len(merged)))
	return nil
}

func (s *Store) read() table {
	var t table

	_ = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		raw := b.Get([]byte("table"))
		if len(raw) == 0 {
			return nil
		}
		return json.Unmarshal(raw, &t)
	})

	return t
}

func (s *Store) write(t table) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return bolt.ErrBucketNotFound
		}
		raw, err := json.Marshal(t)
		if err != nil {
			return err
		}
		return b.Put([]byte("table"), raw)
	})
}

// ForURL resolves the profile for a URL against the live table. Returns the
// zero Profile when no store was initialised (tests, partial startup), which
// the caller treats as "arguments from config only".
func ForURL(rawURL string) (Profile, bool) {
	globalMu.RLock()
	s := global
	globalMu.RUnlock()

	if s == nil {
		return Profile{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.table.match(rawURL)
}
