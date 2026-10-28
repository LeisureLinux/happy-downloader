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

		// Carry the seed (config-file cookie/POT/proxy) into the entries
		// that want it.
		//
		// The proxy goes to YouTube and nowhere else. It used to go to the
		// fallback as well, which meant every site without a profile of its
		// own was reached through it — and a site inside China answers that
		// with a block rather than with the video.
		//
		// So the proxy is a property of a site: the one that needs it asks
		// for it, and an empty proxy means "reach it directly".
		for i := range t {
			if strings.Contains(t[i].HostSuffix, "youtube.com") {
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

		// The fallback gets the credentials but not the proxy: a URL no
		// profile names should still be signed in, and should still be
		// reached the way its own country expects.
		if seed.CookiePath != "" || seed.PotBaseURL != "" {
			for i := range t {
				if t[i].HostSuffix == "" {
					if seed.CookiePath != "" {
						t[i].CookiePath = seed.CookiePath
					}
					if seed.PotBaseURL != "" {
						t[i].PotBaseURL = seed.PotBaseURL
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

	// Move the proxy off the fallback.
	//
	// The proxy used to be the fallback for every site a profile did not
	// name, so a deployment that was seeded before the change has it on the
	// fallback — and every site without a profile of its own is reached
	// through it. Bilibili answers that with 412 rather than with the
	// video.
	//
	// The proxy moves to the YouTube entry, which is the one that was
	// seeded with it in the first place, and the fallback is left to be
	// reached directly. A proxy an operator set on the fallback by hand is
	// left alone: that was a decision, not a leftover.
	if s.moveProxyOffFallback(seed.Proxy) {
		if err := s.write(s.table); err != nil {
			slog.Error("could not write the migrated profiles", slog.Any("err", err))
		}
	}

	globalMu.Lock()
	global = s
	globalMu.Unlock()

	return s, nil
}

// moveProxyOffFallback takes the proxy off the fallback entry and puts it on
// the YouTube entry, when that is where it came from.
//
// It only acts when the fallback's proxy is the one the configuration named —
// a different proxy there was put there by hand, and moving it would undo
// somebody's decision.
//
// It reports whether anything changed.
func (s *Store) moveProxyOffFallback(configuredProxy string) bool {
	if configuredProxy == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	fallbackIndex := -1
	youtubeIndex := -1

	for i, p := range s.table {
		if p.HostSuffix == "" {
			fallbackIndex = i
		}
		if youtubeIndex == -1 && strings.Contains(p.HostSuffix, "youtube.com") {
			youtubeIndex = i
		}
	}

	if fallbackIndex < 0 || youtubeIndex < 0 {
		return false
	}

	fallback := &s.table[fallbackIndex]
	if fallback.Proxy != configuredProxy {
		// Somebody set a different proxy on the fallback. That was a
		// decision, not a leftover.
		return false
	}

	// YouTube keeps the proxy it already has, or the configured one.
	if s.table[youtubeIndex].Proxy == "" {
		s.table[youtubeIndex].Proxy = configuredProxy
	}

	fallback.Proxy = ""

	slog.Info("the proxy was taken off the fallback profile",
		slog.String("proxy", configuredProxy),
		slog.String("to", s.table[youtubeIndex].Name),
	)

	return true
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
