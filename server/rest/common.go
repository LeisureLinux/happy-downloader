package rest

import (
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/livestream"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/queue"

	bolt "go.etcd.io/bbolt"
)

type ContainerArgs struct {
	DB  *bolt.DB
	MDB *kv.Store
	MQ  *queue.MessageQueue
	LM  *livestream.Monitor
	// SourceIndex remembers where the downloaded files came from, so their
	// captions can be fetched later.
	SourceIndex *kv.SourceIndex
}
