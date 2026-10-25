package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	s.lm.Add(req.URL)
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
}

// CookieStatus reports what the server currently has on disk.
//
// It answers "is my cookie export being picked up?" without the user having to
// infer it from a failed download. The cookie names are reported because a
// missing LOGIN_INFO is the usual cause of a guest request, but no value is
// ever included: a cookie value is a live credential.
func (s *Service) CookieStatus() (*CookieStatus, error) {
	if w := cookiewatch.Global(); w != nil {
		st := w.Status()
		return &CookieStatus{
			Exists:     st.Exists,
			SourcePath: st.SourcePath,
			Format:     st.Format,
			Count:      st.Count,
			HttpOnly:   st.HttpOnly,
			Session:    st.Session,
			Expired:    st.Expired,
			Missing:    st.Missing,
			ModifiedAt: st.ModifiedAt,
		}, nil
	}

	// No export configured: report on the converted file if it already exists.
	path := config.CookiesPath()
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return &CookieStatus{Exists: false}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	summary := cookies.Summarize(string(content))

	return &CookieStatus{
		Exists:     true,
		Format:     string(cookies.Detect(content)),
		Count:      summary.Total,
		HttpOnly:   summary.HttpOnly,
		Session:    summary.Session,
		Expired:    summary.Expired,
		Missing:    cookies.Required(summary.Names),
		ModifiedAt: info.ModTime().Format(time.RFC3339),
	}, nil
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

	result := make(chan string, 1)

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	cmd := exec.CommandContext(ctx, config.Instance().Paths.DownloaderPath, "--version")
	go func() {
		stdout, _ := cmd.Output()
		// yt-dlp prints a bare date; some channels prefix a "v". Strip it
		// so the footer does not render "yt-dlp vv2026.10.21".
		v := strings.TrimSpace(string(stdout))
		result <- strings.TrimPrefix(v, "v")
	}()

	select {
	case <-ctx.Done():
		return serverVersion, ServerBuiltAt, "", errors.New("requesting yt-dlp version took too long")
	case res := <-result:
		return serverVersion, ServerBuiltAt, res, nil
	}
}
