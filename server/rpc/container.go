package rpc

import (
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/livestream"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/queue"
		"github.com/go-chi/chi/v5"
)

// Dependency injection container.
func Container(db *kv.Store, mq *queue.MessageQueue, lm *livestream.Monitor) *Service {
	return &Service{
		db: db,
		mq: mq,
		lm: lm,
	}
}

// RPC service must be registered before applying this router!
func ApplyRouter() func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/ws", WebSocket)
		r.Post("/http", Post)
	}
}
