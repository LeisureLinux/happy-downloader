package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/filebrowser"
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
	// There are two exports and the newer one wins, so this says which is in
	// effect — otherwise the footer would show one export's timestamp while
	// the downloads were using the other.
	Active string `json:"active,omitempty"`

	// ActivePath and ActiveModifiedAt describe that export.
	//
	// They are reported directly rather than left for the interface to pick
	// out of Imported and the automatic fields: the choice is made here, and
	// a second implementation of it in the interface is a second answer.
	ActivePath       string `json:"activePath,omitempty"`
	ActiveModifiedAt string `json:"activeModifiedAt,omitempty"`

	// Sites is the file broken down by site, which is the question a person
	// looking at this dialog actually has: which site's session is stale.
	Sites []CookieSite `json:"sites,omitempty"`

	// The imported export, when there is one.
	Imported *ImportedCookie `json:"imported,omitempty"`
}

// CookieSite is the state of one site's session cookie.
//
// Only the session cookie is reported. A cookie file holds hundreds per site
// and almost all of them are disposable — page tokens, experiment flags,
// counters — so a count of them says nothing about whether the site works.
// What does is whether the one cookie that is the session is there and still
// alive.
type CookieSite struct {
	// Site is the label: the name the interface knows the site by, e.g.
	// "youtube", or the domain when it is one nobody knows.
	Site string `json:"site"`
	// Known says whether Site is a site the interface has an icon for.
	Known bool `json:"known"`
	// Domains are the cookie domains that were merged into this row.
	Domains []string `json:"domains"`

	// Cookie is the name of the session cookie, e.g. "SESSDATA".
	//
	// Empty when the site is one whose session cookie this server does not
	// know the name of; the row then says only where the cookies are from,
	// rather than guessing a name and reporting the guess as a fact.
	Cookie string `json:"cookie,omitempty"`

	// Present is whether the file has that cookie: false means the export is
	// signed out for the site, which is why a download comes back a guest.
	Present bool `json:"present"`

	// Expiry is when the cookie dies, empty for a session cookie.
	Expiry string `json:"expiry,omitempty"`

	// Expired is whether that date has passed.
	Expired bool `json:"expired"`
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

	// Which export is in use, and what is in it.
	//
	// This is answered by the same call the downloader makes, so the footer
	// and the downloads cannot disagree about which file is being used.
	active := cookies.Active()
	if active.Path != "" {
		status.Active = active.Kind

		// The source is what a person recognises: the file they uploaded, or
		// the browser export a timer replaces. The converted copy it is read
		// from is an implementation detail.
		status.ActivePath = active.Source
		if status.ActivePath == "" {
			status.ActivePath = active.Path
		}
		if !active.ModifiedAt.IsZero() {
			status.ActiveModifiedAt = active.ModifiedAt.Format(time.RFC3339)
		}
		status.Sites = readSites(active.Path)
	}

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
			if status.Active == "" {
				status.Active = cookies.KindImported
			}
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

// readSites breaks the active cookie file down by site.
//
// The file is one list for every site a browser has visited, and the question
// a person has is per site: which session is stale. A single figure for the
// whole file cannot answer it — a file can be enormous and hold nothing for
// the site that is failing.
//
// The domains are merged by site, because a site sets cookies on several of
// them: "douyin.com" and "www.douyin.com" are one account, and two rows for
// them would read as two problems. The site name comes from the one place that
// knows how a host maps to a site, so the icon is the same one the file
// browser shows. A host nobody knows keeps its own domain as the label: the
// fallback would reduce "accounts.google.com" to "accounts", which is not a
// site and reads as one.
//
// A cookie is a credential, so only the shape is returned: how many, when the
// newest one expires, and whether the site's login cookies are there. No value
// is ever read out of the file.
func readSites(path string) []CookieSite {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	merged := map[string]*CookieSite{}
	order := []string{}

	for _, s := range cookies.SummarizeDomains(string(content)) {
		site, known := filebrowser.SiteOfHost(s.Domain)

		label := site
		if !known {
			label = registrableDomain(s.Domain)
		}

		row, ok := merged[label]
		if !ok {
			row = &CookieSite{Site: label, Known: known}
			merged[label] = row
			order = append(order, label)
		}

		row.Domains = append(row.Domains, s.Domain)

		if !s.KeyCookie.Known {
			continue
		}

		// The domains of one site disagree about the session cookie: it is
		// set on the site's main domain and not on the delivery host, and a
		// dump can hold one without the other. The present one wins, and a
		// live date wins over a passed one.
		switch {
		case row.Cookie == "":
			row.Cookie = s.KeyCookie.Name
			row.Present = s.KeyCookie.Present
			row.Expired = s.KeyCookie.Expired
			if !s.KeyCookie.Expiry.IsZero() {
				row.Expiry = s.KeyCookie.Expiry.Format(time.RFC3339)
			}
		case s.KeyCookie.Present && !row.Present:
			row.Present = true
			row.Expired = s.KeyCookie.Expired
			row.Expiry = ""
			if !s.KeyCookie.Expiry.IsZero() {
				row.Expiry = s.KeyCookie.Expiry.Format(time.RFC3339)
			}
		}
	}

	// Sites whose session is live first, then the ones signed out, then the
	// ones nobody knows a cookie name for. A person opens this to find the
	// site that is not working, so the broken ones belong at the top of what
	// is left.
	out := make([]CookieSite, 0, len(order))
	for _, label := range order {
		out = append(out, *merged[label])
	}

	sort.SliceStable(out, func(i, j int) bool {
		rank := func(c CookieSite) int {
			switch {
			case c.Cookie == "":
				return 2
			case !c.Present || c.Expired:
				return 0
			default:
				return 1
			}
		}

		if rank(out[i]) != rank(out[j]) {
			return rank(out[i]) < rank(out[j])
		}
		return out[i].Site < out[j].Site
	})

	return out
}

// registrableDomain reduces a host to the part that names the site.
//
// It is the last two labels, which is right for "accounts.google.com" and for
// "www.douyin.com". It is wrong for "example.co.uk", and that is accepted: the
// label is only used for hosts nobody has an icon for, and a slightly short
// name is better than a different row for every subdomain.
func registrableDomain(host string) string {
	parts := strings.Split(strings.Trim(host, "."), ".")
	if len(parts) <= 2 {
		return strings.Join(parts, ".")
	}

	return strings.Join(parts[len(parts)-2:], ".")
}
