package archive

import (
	"database/sql"

	"github.com/LeisureLinux/happy-downloader/v4/server/archive/domain"
	"github.com/go-chi/chi/v5"
)

// alias type
// TODO: remove after refactoring
type Service = domain.Service
type Entity = domain.ArchiveEntry

func ApplyRouter(db *sql.DB) func(chi.Router) {
	handler, _ := Container(db)
	return handler.ApplyRouter()
}
