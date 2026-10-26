package downloaders

import (
	"github.com/LeisureLinux/happy-downloader/v4/server/common"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
)

type Downloader interface {
	Start() error
	Stop() error
	Status() internal.ProcessSnapshot

	SetOutput(output internal.DownloadOutput)
	SetProgress(progress internal.DownloadProgress)
	SetMetadata(fetcher func(url string) (*common.DownloadMetadata, error))
	SetPending(p bool)

	IsCompleted() bool

	UpdateSavedFilePath(path string)
	UpdateOutputDimensions(width, height float64)

	RestoreFromSnapshot(*internal.ProcessSnapshot) error

	GetId() string
	GetUrl() string
}
