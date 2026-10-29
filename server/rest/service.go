package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/cookies"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/cookiewatch"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/livestream"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/queue"
	"github.com/LeisureLinux/happy-downloader/v4/server/playlist"
	"github.com/google/uuid"

	bolt "go.etcd.io/bbolt"
)

type Service struct {
	mdb *kv.Store
	db  *bolt.DB
	mq  *queue.MessageQueue
	lm  *livestream.Monitor
}

func NewService(
	mdb *kv.Store,
	db *bolt.DB,
	mq *queue.MessageQueue,
	lm *livestream.Monitor,
) *Service {
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("templates"))
		return err
	})
	return &Service{
		mdb: mdb,
		db:  db,
		mq:  mq,
		lm:  lm,
	}
}

func (s *Service) Exec(req internal.DownloadRequest) (string, error) {
	d := downloaders.NewGenericDownload(req.URL, req.Params)
	d.SetOutput(internal.DownloadOutput{
		Path:     req.Path,
		Filename: req.Rename,
	})

	id := s.mdb.Set(d)
	s.mq.Publish(d)

	return id, nil
}

func (s *Service) ExecPlaylist(req internal.DownloadRequest) error {
	return playlist.PlaylistDetect(req, s.mq, s.mdb)
}

func (s *Service) ExecLivestream(req internal.DownloadRequest) {
	s.lm.Add(req.URL, req.RecordMinutes)
}

func (s *Service) Running(ctx context.Context) (*[]internal.ProcessSnapshot, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
		return s.mdb.All(), nil
	}
}

// CookieStatus describes the stored cookie file without exposing any value.
//
// It exists so the UI can answer "is my cookie file being picked up?" without
// the user having to infer it from a download failing. The names are reported
// because a missing LOGIN_INFO is the usual cause of a guest request, but the
// values never leave the server: a cookie value is a live credential.
type CookieStatus struct {
	Exists     bool     `json:"exists"`
	SourcePath string   `json:"sourcePath,omitempty"`
	Format     string   `json:"format,omitempty"`
	Count      int      `json:"count"`
	HttpOnly   int      `json:"httpOnly"`
	Session    int      `json:"session"`
	Expired    int      `json:"expired"`
	Missing    []string `json:"missing,omitempty"`
	ModifiedAt string   `json:"modifiedAt,omitempty"`

	// The cookie the downloads are actually using.
	//
	// The imported export takes precedence over the automatic one, and this
	// says which is in effect — otherwise the footer would show the
	// automatic export's timestamp while the downloads were using
	// something else entirely.
	Active string `json:"active,omitempty"`

	// The imported export, when there is one.
	Imported *ImportedCookie `json:"imported,omitempty"`
}

// ImportedCookie describes the cookie export the user uploaded.
type ImportedCookie struct {
	// Path is where it was written.
	Path string `json:"path"`
	// Format is the shape it was recognised as.
	Format string `json:"format"`
	// Count is how many cookies it holds.
	Count int `json:"count"`
	// ModifiedAt is when it was uploaded.
	ModifiedAt string `json:"modifiedAt"`
	// Failures is how many times a download using it was refused.
	Failures int `json:"failures"`
}

// CookieStatus reports what the server currently has on disk.
//
// It answers "is my cookie export being picked up?" without the user having to
// infer it from a failed download. The cookie names are reported because a
// missing LOGIN_INFO is the usual cause of a guest request, but no value is
// ever included: a cookie value is a live credential.
func (s *Service) CookieStatus() (*CookieStatus, error) {
	status := &CookieStatus{}

	// The imported export, when there is one.
	//
	// It is the cookie the downloads use, so it is reported first and the
	// automatic export is only the fallback underneath it.
	if imported := cookies.ImportedGlobal(); imported != nil {
		cur := imported.Current()
		if cur != nil {
			status.Imported = &ImportedCookie{
				Path:       cur.Path,
				Format:     cur.Format,
				Count:      cur.Count,
				ModifiedAt: cur.ModifiedAt.Format(time.RFC3339),
				Failures:   cur.Failures,
			}
			status.Active = "imported"
		}
	}

	// The automatic export: the browser profile read by the refresh script
	// and copied onto the host by a timer.
	if w := cookiewatch.Global(); w != nil {
		st := w.Status()
		status.Exists = st.Exists
		status.SourcePath = st.SourcePath
		status.Format = st.Format
		status.Count = st.Count
		status.HttpOnly = st.HttpOnly
		status.Session = st.Session
		status.Expired = st.Expired
		status.Missing = st.Missing
		status.ModifiedAt = st.ModifiedAt

		if status.Active == "" && st.Exists {
			status.Active = "automatic"
		}

		return status, nil
	}

	// No watcher: report on the converted file if an export is configured and
	// has already been converted.
	source := config.ResolvedCookieSource()
	if source == "" {
		return status, nil
	}

	path := config.CookiesPath(source)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return status, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	summary := cookies.Summarize(string(content))

	status.Exists = true
	status.SourcePath = source
	status.Format = string(cookies.Detect(content))
	status.Count = summary.Total
	status.HttpOnly = summary.HttpOnly
	status.Session = summary.Session
	status.Expired = summary.Expired
	status.Missing = cookies.Required(summary.Names)
	status.ModifiedAt = info.ModTime().Format(time.RFC3339)

	if status.Active == "" {
		status.Active = "automatic"
	}

	return status, nil
}

func (s *Service) SaveTemplate(ctx context.Context, template *internal.CustomTemplate) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("templates"))
		v, err := json.Marshal(template)
		if err != nil {
			return err
		}
		return b.Put([]byte(uuid.NewString()), v)
	})
}

func (s *Service) GetTemplates(ctx context.Context) (*[]internal.CustomTemplate, error) {
	templates := make([]internal.CustomTemplate, 0)

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("templates"))
		if b == nil {
			return nil // bucket vuoto, restituisco lista vuota
		}

		return b.ForEach(func(k, v []byte) error {
			var t internal.CustomTemplate
			if err := json.Unmarshal(v, &t); err != nil {
				return err
			}
			templates = append(templates, t)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return &templates, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, t *internal.CustomTemplate) (*internal.CustomTemplate, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	err = s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("templates"))
		if b == nil {
			return fmt.Errorf("bucket templates not found")
		}
		return b.Put([]byte(t.Id), data)
	})

	if err != nil {
		return nil, err
	}

	return t, nil
}

func (s *Service) DeleteTemplate(ctx context.Context, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("templates"))
		return b.Delete([]byte(id))
	})
}

// ServerVersion is the version of this server, injected at build time with
// -ldflags "-X .../server/rest.ServerVersion=4.2.1". It defaults to "dev" so
// a plain "go build" is visibly not a release build.
//
// The value carries no "v" prefix: the UI renders it as "v<version>@<built>",
// so a tag-shaped value would render as "vv4.2.1".
var ServerVersion = "dev"

// ServerBuiltAt is the build timestamp (unix seconds), injected the same way
// with -X .../server/rest.ServerBuiltAt=$(date +%s). It defaults to "0", which
// the UI renders as no timestamp rather than a misleading one.
//
// Without it the only way to tell a stale binary from a current one is the
// file mtime, which is exactly the check that gets skipped.
var ServerBuiltAt = "0"

func (s *Service) GetVersion(ctx context.Context) (string, string, string, error) {
	// The UI renders "UI v<version>" and "yt-dlp v<version>", so
	// neither may carry its own "v": a tag-shaped "v4.2.0" injected
	// into ServerVersion would render as "UI vv4.2.0".
	serverVersion := strings.TrimPrefix(ServerVersion, "v")

	// The yt-dlp version is remembered rather than asked for every time.
	// Asking is a process spawn, and the standalone binary takes four
	// seconds to start — which the page waited for, on every load, because
	// the footer shows the version.
	return serverVersion, ServerBuiltAt, ytDlpVersion(ctx), nil
}
