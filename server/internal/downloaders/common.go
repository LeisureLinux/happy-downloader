package downloaders

import (
	"log/slog"
	"sync"

	"github.com/LeisureLinux/happy-downloader/v4/server/common"
)

type DownloaderBase struct {
	Id        string
	URL       string
	Metadata  common.DownloadMetadata
	Pending   bool
	Completed bool
	mutex     sync.Mutex

	// FailureReason is what yt-dlp said when it gave up.
	//
	// The exit code is all the process tells us, and "exit status 1" is not
	// an answer to anything. The reason is the line yt-dlp wrote to stderr
	// before it exited, kept so that the download log can say it.
	FailureReason string

	// FromIP is the address the download was asked for from.
	//
	// It is recorded because "which machine started this" is a question the
	// access log answers for a request and nothing answered for a download:
	// the download outlives the request that created it, and by the time it
	// fails the request is long gone from the log.
	FromIP string
}

// SetFailureReason records why yt-dlp gave up.
func (d *DownloaderBase) SetFailureReason(reason string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.FailureReason = reason
}

// GetFailureReason is why yt-dlp gave up, when it said.
func (d *DownloaderBase) GetFailureReason() string {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.FailureReason
}

// SetFromIP records the address the download was asked for from.
func (d *DownloaderBase) SetFromIP(ip string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.FromIP = ip
}

// GetFromIP is the address the download was asked for from.
func (d *DownloaderBase) GetFromIP() string {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.FromIP
}

func (d *DownloaderBase) FetchMetadata(fetcher func(url string) (*common.DownloadMetadata, error)) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	meta, err := fetcher(d.URL)
	if err != nil {
		slog.Error("failed to retrieve metadata", slog.String("err", err.Error()))
		return
	}

	d.Metadata = *meta
}

func (d *DownloaderBase) SetPending(p bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Pending = p
}

func (d *DownloaderBase) Complete() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Completed = true
}

// Reopen clears the finished mark, so the download can run again.
//
// It exists for the one case that is not a retry: a recording that was
// stopped on purpose, which the operator wants to start again on the same
// row. The files it already produced are kept — they are in the download
// directory and in the source index, and nothing here touches them.
func (d *DownloaderBase) Reopen() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Completed = false
}
