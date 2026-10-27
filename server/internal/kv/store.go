package kv

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/queue"

	bolt "go.etcd.io/bbolt"
)

var (
	bucket      = []byte("downloads")
	memDbEvents = make(chan downloaders.Downloader, runtime.NumCPU())
)

// In-Memory Thread-Safe Key-Value Storage with optional persistence
type Store struct {
	db    *bolt.DB
	table map[string]downloaders.Downloader
	mu    sync.RWMutex
}

func NewStore(db *bolt.DB, snaptshotInteval time.Duration) (*Store, error) {
	// init bucket
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	})
	if err != nil {
		return nil, err
	}

	s := &Store{
		db:    db,
		table: make(map[string]downloaders.Downloader),
	}

	go func() {
		ticker := time.NewTicker(snaptshotInteval)
		for range ticker.C {
			s.Snapshot()
		}
	}()

	return s, err
}

// Get a process pointer given its id
func (m *Store) Get(id string) (downloaders.Downloader, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.table[id]
	if !ok {
		return nil, errors.New("no process found for the given key")
	}

	return entry, nil
}

// Store a pointer of a process and return its id
func (m *Store) Set(d downloaders.Downloader) string {
	m.mu.Lock()
	m.table[d.GetId()] = d
	m.mu.Unlock()

	return d.GetId()
}

// Removes a process progress, given the process id
func (m *Store) Delete(id string) {
	m.mu.Lock()
	delete(m.table, id)
	m.mu.Unlock()

	m.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		b.Delete([]byte(id))
		return nil
	})
}

func (m *Store) Keys() *[]string {
	var running []string

	m.mu.RLock()
	defer m.mu.RUnlock()

	for id := range m.table {
		running = append(running, id)
	}

	return &running
}

// Returns a slice of all currently stored processes progess
func (m *Store) All() *[]internal.ProcessSnapshot {
	running := []internal.ProcessSnapshot{}

	m.mu.RLock()
	for _, v := range m.table {
		snap := v.Status()
		// Fill in what only the filesystem knows. The metadata estimate is
		// what the source offered at its best, which is not what was written
		// once a resolution cap or audio-only choice applied.
		// Assign both fields on every pass rather than only on success: they
		// are persisted with the snapshot, so a value written while the file
		// existed survives its deletion unless it is cleared here.
		snap.Output.FileExists = false
		snap.Output.FileSize = 0
		if snap.Output.SavedFilePath != "" {
			if info, err := os.Stat(snap.Output.SavedFilePath); err == nil && !info.IsDir() {
				snap.Output.FileSize = info.Size()
				snap.Output.FileExists = true
			}
		}
		running = append(running, snap)
	}
	m.mu.RUnlock()

	return &running
}

// Restore a persisted state
func (m *Store) Restore(mq *queue.MessageQueue) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var snapshot []internal.ProcessSnapshot

	m.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		return b.ForEach(func(k, v []byte) error {
			var snap internal.ProcessSnapshot
			if err := json.Unmarshal(v, &snap); err != nil {
				return err
			}
			snapshot = append(snapshot, snap)
			return nil
		})
	})

	for _, snap := range snapshot {
		var restored downloaders.Downloader
		if snap.DownloaderName == "generic" {
			d := downloaders.NewGenericDownload("", []string{})
			err := d.RestoreFromSnapshot(&snap)
			if err != nil {
				continue
			}
			restored = d
			m.table[snap.Id] = restored

			// A paused download stays paused across a restart. It was
			// stopped on purpose, and restarting the server is not a
			// request to start it again — the partial file is still there
			// for when the user asks.
			if snap.Progress.Status == internal.StatusPaused {
				continue
			}

			if !restored.(*downloaders.GenericDownloader).DownloaderBase.Completed {
				mq.Publish(restored)
			}
		}
	}
}

func (m *Store) EventListener() {
	for p := range memDbEvents {
		if p.Status().DownloaderName == "livestream" {
			slog.Info("compacting Store", slog.String("id", p.GetId()))
			m.Delete(p.GetId())
		}
	}
}

func (m *Store) Snapshot() error {
	slog.Debug("snapshotting downloads state")

	running := m.All()

	return m.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		for _, v := range *running {
			data, err := json.Marshal(v)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(v.Id), data); err != nil {
				return err
			}
		}
		return nil
	})
}
