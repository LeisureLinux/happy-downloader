package kv

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	bolt "go.etcd.io/bbolt"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	db, err := bolt.Open(filepath.Join(t.TempDir(), "test.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	s, err := NewStore(db, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestKeyForURL(t *testing.T) {
	cases := []struct {
		a, b string
		same bool
	}{
		{
			"https://x.com/user/status/123/video/1?a=abc&s=46",
			"https://x.com/user/status/123/video/1",
			true,
		},
		{
			"https://youtu.be/abcdefghijk?si=sharetoken",
			"https://www.youtube.com/watch?v=abcdefghijk",
			true,
		},
		{
			"https://example.com/a.mp4",
			"https://example.com/b.mp4",
			false,
		},
		{"", "", false},
	}

	for _, c := range cases {
		a, b := KeyForURL(c.a), KeyForURL(c.b)
		if (a == b && a != "") != c.same {
			t.Errorf("KeyForURL(%q)=%q, KeyForURL(%q)=%q, same=%v", c.a, a, c.b, b, c.same)
		}
	}
}

func TestFindActiveByURL(t *testing.T) {
	s := newTestStore(t)

	first := downloaders.NewGenericDownload("https://x.com/user/status/123/video/1?a=abc&s=46", nil).(*downloaders.GenericDownloader)
	s.Set(first)

	if _, ok := s.FindActiveByURL("https://x.com/user/status/123/video/1"); !ok {
		t.Fatal("the same video pasted without tracking parameters was not recognised")
	}
	if got, _ := s.FindActiveByURL("https://x.com/user/status/123/video/1"); got.GetId() != first.GetId() {
		t.Fatal("the wrong download was returned")
	}
	if _, ok := s.FindActiveByURL("https://x.com/user/status/999/video/1"); ok {
		t.Fatal("a different video matched")
	}
	if _, ok := s.FindActiveByURL(""); ok {
		t.Fatal("an empty URL matched")
	}

	// A finished download is not a duplicate: its file is in the browser,
	// and asking for the video again is a new request.
	first.SetProgress(internal.DownloadProgress{Status: internal.StatusCompleted})
	first.DownloaderBase.Complete()
	if _, ok := s.FindActiveByURL("https://x.com/user/status/123/video/1"); ok {
		t.Fatal("a completed download was treated as an active duplicate")
	}
}
