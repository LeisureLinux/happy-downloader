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

	// IsPaused reports whether this download was stopped on purpose and is
	// waiting to be resumed.
	IsPaused() bool

	UpdateSavedFilePath(path string)
	UpdateOutputDimensions(width, height float64)
	SetDuration(seconds float64)
	SetCaptions(captions []internal.CaptionTrack)

	RestoreFromSnapshot(*internal.ProcessSnapshot) error

	GetId() string
	GetUrl() string

	// SetFromIP records the address the download was asked for from.
	SetFromIP(ip string)
	GetFromIP() string

	// SetFailureReason records what yt-dlp said when it gave up, so that
	// the download log can say more than the exit code.
	SetFailureReason(reason string)
	GetFailureReason() string
}
