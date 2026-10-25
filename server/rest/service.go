package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/config"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/internal"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/internal/cookies"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/internal/downloaders"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/internal/kv"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/internal/livestream"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/internal/queue"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/playlist"

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

func (s *Service) GetCookies(ctx context.Context) ([]byte, error) {
	fd, err := os.Open(config.CookiesPath())
	if err != nil {
		return nil, err
	}

	defer fd.Close()

	cookies, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}

	return cookies, nil
}

// SetCookies accepts a browser cookie export in any of the shapes extensions
// produce and stores it as Netscape cookies.txt, which is what yt-dlp reads.
//
// The format is detected from the content, so a JSON export from a browser
// extension and a hand-written cookies.txt both arrive as-is and work. This is
// what makes the file uploadable to a headless host: there is no browser there
// to ask for a different format.
func (s *Service) SetCookies(ctx context.Context, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return os.Remove(config.CookiesPath())
	}

	converted, format, err := cookies.Convert([]byte(raw))
	if err != nil {
		return err
	}

	// Cookie values never reach the log. Only the shape and the cookie names do,
	// which is enough to tell a login session from a guest one.
	summary := cookies.Summarize(converted)
	slog.Info("cookie file updated",
		slog.String("format", string(format)),
		slog.Int("count", summary.Total),
		slog.Int("httpOnly", summary.HttpOnly),
		slog.Int("session", summary.Session),
		slog.Int("expired", summary.Expired),
	)

	if missing := cookies.Required(summary.Names); len(missing) > 0 {
		slog.Warn("cookie file is missing YouTube login cookies; "+
			"requests may be treated as a guest",
			slog.Any("missing", missing))
	}

	if err := os.MkdirAll(filepath.Dir(config.CookiesPath()), 0o755); err != nil {
		return err
	}

	// 0600: this file is a live session credential.
	fd, err := os.OpenFile(config.CookiesPath(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer fd.Close()

	_, err = fd.WriteString(converted)
	return err
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

func (s *Service) GetVersion(ctx context.Context) (string, string, error) {
	//TODO: load from realease properties file, or anything else outside code
	const CURRENT_RPC_VERSION = "4.0.0"

	result := make(chan string, 1)

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	cmd := exec.CommandContext(ctx, config.Instance().Paths.DownloaderPath, "--version")
	go func() {
		stdout, _ := cmd.Output()
		result <- string(stdout)
	}()

	select {
	case <-ctx.Done():
		return CURRENT_RPC_VERSION, "", errors.New("requesting yt-dlp version took too long")
	case res := <-result:
		return CURRENT_RPC_VERSION, res, nil
	}
}
