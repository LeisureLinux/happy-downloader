package archive

import (
	"database/sql"
	"sync"

	"github.com/LeisureLinux/happy-downloader/v4/server/archive/domain"
	"github.com/LeisureLinux/happy-downloader/v4/server/archive/repository"
	"github.com/LeisureLinux/happy-downloader/v4/server/archive/rest"
	"github.com/LeisureLinux/happy-downloader/v4/server/archive/service"
)

var (
	repo domain.Repository
	svc  domain.Service
	hand domain.RestHandler

	repoOnce sync.Once
	svcOnce  sync.Once
	handOnce sync.Once
)

func provideRepository(db *sql.DB) domain.Repository {
	repoOnce.Do(func() {
		repo = repository.New(db)
	})
	return repo
}

func provideService(r domain.Repository) domain.Service {
	svcOnce.Do(func() {
		svc = service.New(r)
	})
	return svc
}

func provideHandler(s domain.Service) domain.RestHandler {
	handOnce.Do(func() {
		hand = rest.New(s)
	})
	return hand
}
