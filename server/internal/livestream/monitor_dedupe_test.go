package livestream

import (
	"path/filepath"
	"testing"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/queue"
	bolt "go.etcd.io/bbolt"
)

func newTestMonitor(t *testing.T) *Monitor {
	t.Helper()

	db, err := bolt.Open(filepath.Join(t.TempDir(), "test.db"), 0o600, nil)
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	c := &config.Config{}
	c.Server.QueueSize = 1
	config.SetInstance(c)
	t.Cleanup(func() { config.SetInstance(&config.Config{}) })

	mq, err := queue.NewMessageQueue()
	if err != nil {
		t.Fatalf("creating the queue: %v", err)
	}

	// The downloader is never started in these tests, so the store only has
	// to exist.
	return NewMonitor(mq, nil, db)
}

// TestAddFromWatchesAURLOnlyOnce is the duplicate recording bug.
//
// The monitor re-adds everything it had saved when the server starts, so a
// stream that was being recorded across a restart is already being watched
// when the person asks for it again. Adding it a second time replaced the
// entry in the map and left the first watcher running with nothing pointing
// at it — and when that one decided the stream was live it published a
// recording of its own. Two rows, both live, both writing a file.
func TestAddFromWatchesAURLOnlyOnce(t *testing.T) {
	m := newTestMonitor(t)

	const url = "https://example.test/live"

	// The stream is registered without being started: Start would look for a
	// live stream, which is not what this is about.
	first := New(url, m.done, m.mq, m.store)

	m.mu.Lock()
	m.streams[url] = first
	m.mu.Unlock()

	m.AddFrom(url, 0, "127.0.0.1", "")

	m.mu.Lock()
	got := m.streams[url]
	count := len(m.streams)
	m.mu.Unlock()

	if count != 1 {
		t.Errorf("want one watched stream, got %d", count)
	}

	if got != first {
		t.Error("the existing watcher was replaced instead of being kept")
	}
}

// TestForgetLetsTheSameURLBeWatchedAgain covers the other half: stopping a
// recording must not make the URL unrecordable.
func TestForgetLetsTheSameURLBeWatchedAgain(t *testing.T) {
	m := newTestMonitor(t)

	const url = "https://example.test/live"

	m.mu.Lock()
	m.streams[url] = New(url, m.done, m.mq, m.store)
	m.mu.Unlock()

	m.Forget(url)

	m.mu.Lock()
	count := len(m.streams)
	m.mu.Unlock()

	if count != 0 {
		t.Fatalf("want the stream forgotten, got %d", count)
	}
}
