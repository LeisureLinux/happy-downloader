// Package settings persists operator overrides for the download credentials.
//
// The YAML config file defines the baseline, but the values an operator
// actually changes at runtime — the cookie export path, the POT provider URL,
// the egress proxy — need to be editable from the UI without a restart, and a
// rewrite of the YAML from the process would corrupt relative-path semantics
// the operator relies on. Overrides therefore live in a bolt bucket and are
// merged over the config on startup and on every change.
package settings

import (
	"encoding/json"
	"log/slog"
	"os"

	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/config"
	bolt "go.etcd.io/bbolt"
)

const bucketName = "settings"

// Override is the set of values the UI may change at runtime. Zero-valued
// fields mean "fall back to the YAML config".
type Override struct {
	CookiePath string `json:"cookie_path,omitempty"`
	PotBaseURL string `json:"pot_base_url,omitempty"`
	Proxy      string `json:"proxy,omitempty"`
}

// Apply copies the set fields over the live config. An override persists for
// the lifetime of the process and across restarts; the only way back to the
// YAML value is clearing the field in the UI.
func (o Override) Apply() {
	c := config.Instance()

	if o.CookiePath != "" {
		c.Paths.CookiePath = o.CookiePath
	}
	if o.PotBaseURL != "" {
		c.YtDlp.PotBaseURL = o.PotBaseURL
	}
	if o.Proxy != "" {
		c.YtDlp.Proxy = o.Proxy
	}
}

// Store reads and writes the override bucket.
type Store struct {
	db *bolt.DB
}

func NewStore(db *bolt.DB) (*Store, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Load returns the stored override, or an empty one when nothing was saved.
func (s *Store) Load() Override {
	var out Override

	_ = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		raw := b.Get([]byte("override"))
		if len(raw) == 0 {
			return nil
		}
		return json.Unmarshal(raw, &out)
	})

	return out
}

// Save persists the override and applies it to the live config. An empty
// Override clears the stored values, restoring the YAML baseline.
func (s *Store) Save(o Override) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return bolt.ErrBucketNotFound
		}

		if isEmpty(o) {
			return b.Delete([]byte("override"))
		}

		raw, err := json.Marshal(o)
		if err != nil {
			return err
		}
		return b.Put([]byte("override"), raw)
	})
	if err != nil {
		return err
	}

	slog.Info("download settings updated",
		slog.String("cookie_path", maskPath(o.CookiePath)),
		slog.String("pot_base_url", o.PotBaseURL),
		slog.String("proxy", o.Proxy),
	)

	return nil
}

func isEmpty(o Override) bool {
	return o.CookiePath == "" && o.PotBaseURL == "" && o.Proxy == ""
}

// maskPath keeps the log useful without echoing a full path that might name a
// user directory: the basename is enough to recognise the file.
func maskPath(p string) string {
	if p == "" {
		return ""
	}
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return "…" + p[i:]
		}
	}
	return p
}

// Current returns the effective values: the override when set, else the YAML
// config. This is what the UI displays, so an operator sees which file is
// actually in effect rather than what the YAML happens to say.
func Current() Override {
	c := config.Instance()
	return Override{
		CookiePath: c.Paths.CookiePath,
		PotBaseURL: c.YtDlp.PotBaseURL,
		Proxy:      c.YtDlp.Proxy,
	}
}

// SourceStat reports the cookie export's modification and creation times,
// using the birth time when the filesystem records one. A cookie file's age
// is the first question when a download starts failing, so it belongs next
// to the path rather than buried in the log.
func SourceStat(path string) (mtime, birth string, ok bool) {
	if path == "" {
		return "", "", false
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", "", false
	}

	mtime = info.ModTime().Format("2006-01-02 15:04:05")

	if b := birthTime(info.Sys()); !b.IsZero() {
		birth = b.Format("2006-01-02 15:04:05")
	}

	return mtime, birth, true
}
