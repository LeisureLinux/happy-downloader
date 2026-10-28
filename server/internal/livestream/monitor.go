package livestream

import (
	"errors"
	"sync"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/queue"
	bolt "go.etcd.io/bbolt"
)

var bucket = []byte("livestreams")

type Monitor struct {
	db      *bolt.DB
	store   *kv.Store              // where the just started livestream will be published
	mq      *queue.MessageQueue    // where the just started livestream will be published
	streams map[string]*LiveStream // keeps track of the livestreams
	done    chan *LiveStream       // to signal individual processes completition
	mu      sync.Mutex             // guards streams, which the schedule and the callers both touch
}

func NewMonitor(mq *queue.MessageQueue, store *kv.Store, db *bolt.DB) *Monitor {
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	})

	return &Monitor{
		mq:      mq,
		db:      db,
		store:   store,
		streams: make(map[string]*LiveStream),
		done:    make(chan *LiveStream),
	}
}

// Detect each livestream completition, if done detach it from the monitor.
//
// A stream that was killed is not detached here: the kill sends on the same
// channel, and forgetting it meant the monitor no longer knew the stream was
// there — so it could not be stopped, and its status was no longer reported.
//
// The two are told apart by whether the stream had already published itself
// to the queue. A stream that is downloading is still known; one that was
// only being watched for is forgotten, because there is nothing left of it.
func (m *Monitor) Schedule() {
	for l := range m.done {
		m.mu.Lock()
		recording := l.published
		m.mu.Unlock()

		if recording {
			// The stream is downloading. It stays in the monitor so that it
			// can be stopped and so that its state is reported.
			continue
		}

		m.mu.Lock()
		delete(m.streams, l.url)
		m.mu.Unlock()

		m.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucket)
			return b.Delete([]byte(l.url))
		})
	}
}

func (m *Monitor) Add(url string) {
	ls := New(url, m.done, m.mq, m.store)

	go ls.Start()

	m.mu.Lock()
	m.streams[url] = ls
	m.mu.Unlock()

	m.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		return b.Put([]byte(url), []byte{})
	})
}

func (m *Monitor) Remove(url string) error {
	m.mu.Lock()
	stream, ok := m.streams[url]
	m.mu.Unlock()

	if !ok {
		return errors.New("no such livestream")
	}

	return stream.Kill()
}

func (m *Monitor) RemoveAll() error {
	m.mu.Lock()
	streams := make([]*LiveStream, 0, len(m.streams))
	for _, v := range m.streams {
		streams = append(streams, v)
	}
	m.mu.Unlock()

	for _, v := range streams {
		if err := v.Kill(); err != nil {
			return err
		}
	}
	return nil
}

func (m *Monitor) Status() LiveStreamStatus {
	status := make(LiveStreamStatus)

	m.mu.Lock()
	defer m.mu.Unlock()

	for k, v := range m.streams {
		status[k] = Status{
			Status:   v.status,
			WaitTime: v.waitTime,
			LiveDate: v.liveDate,
		}
	}

	return status
}

// Restore a saved state and resume the monitored livestreams
func (m *Monitor) Restore() error {
	return m.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		return b.ForEach(func(k, v []byte) error {
			m.Add(string(k))
			return nil
		})
	})
}
