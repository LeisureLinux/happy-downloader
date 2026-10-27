package internal

import (
	"github.com/LeisureLinux/happy-downloader/v4/server/common"
)

// Used to unmarshall yt-dlp progress
type ProgressTemplate struct {
	Percentage string  `json:"percentage"`
	Speed      float64 `json:"speed"`
	Size       string  `json:"size"`
	Eta        float64 `json:"eta"`
}

type PostprocessTemplate struct {
	FilePath string  `json:"filepath"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
}

// Defines where and how the download needs to be saved
type DownloadOutput struct {
	Path          string
	Filename      string
	SavedFilePath string `json:"savedFilePath"`
	// FileSize is the size of the file actually written, not the estimate the
	// metadata carried for the best available format. Those differ whenever a
	// resolution cap or an audio-only choice is applied, and showing the
	// estimate next to a file that is a tenth of the size is worse than
	// showing nothing.
	FileSize int64 `json:"fileSize,omitempty"`
	// FileExists reports whether the saved file is still on disk. The list
	// outlives the files: an entry whose file was removed by hand would
	// otherwise offer play and download buttons that 404.
	FileExists bool `json:"fileExists"`
	// Width and Height are the dimensions actually written, captured at
	// postprocess. The metadata's resolution describes the best format the
	// source offers, which is not what a capped or audio-only download
	// produced.
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

// Progress for the Running call
type DownloadProgress struct {
	Status     int     `json:"process_status"`
	Percentage string  `json:"percentage"`
	Speed      float64 `json:"speed"`
	ETA        float64 `json:"eta"`
}

// struct representing the response sent to the client
// as JSON-RPC result field
type ProcessSnapshot struct {
	Id string `json:"id"`
	// The URL as requested. Info.URL is the same value only once metadata has
	// been fetched; restoring from it meant an entry whose metadata stage never
	// completed came back with no URL and could not be retried.
	URL            string                  `json:"url"`
	Progress       DownloadProgress        `json:"progress"`
	Info           common.DownloadMetadata `json:"info"`
	Output         DownloadOutput          `json:"output"`
	Params         []string                `json:"params"`
	DownloaderName string                  `json:"downloader_name"`

	// Duration in seconds, from the probe. Known before a byte is transferred,
	// so the queue can show it without reading the finished file.
	Duration float64 `json:"duration,omitempty"`

	// The caption tracks the source offered. Kept so the player can offer the
	// languages that were actually available, rather than guessing from the
	// filename.
	Captions []CaptionTrack `json:"captions,omitempty"`
}

// CaptionTrack is one caption language the source offers.
type CaptionTrack struct {
	// Language is the code YouTube uses, e.g. "en" or "zh-Hans".
	Language string `json:"language"`
	// Name is the human-readable label, when the source provides one.
	Name string `json:"name,omitempty"`
	// Automatic marks a generated track rather than an authored one.
	Automatic bool `json:"automatic,omitempty"`
}

// struct representing the current status of the memoryDB
// used for serializaton/persistence reasons
type Session struct {
	Snapshots []ProcessSnapshot `json:"processes"`
}

// struct representing the intent to stop a specific process
type AbortRequest struct {
	Id string `json:"id"`
}

// struct representing the intent to start a download
type DownloadRequest struct {
	Id     string
	URL    string   `json:"url"`
	Path   string   `json:"path"`
	Rename string   `json:"rename"`
	Params []string `json:"params"`
}

// struct representing request of creating a netscape cookies file
type SetCookiesRequest struct {
	Cookies string `json:"cookies"`
}

// represents a user defined collection of yt-dlp arguments
type CustomTemplate struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

const (
	StatusPending = iota
	StatusDownloading
	StatusCompleted
	StatusErrored
	StatusLiveStream
	// StatusPaused is a download that was stopped on purpose and is waiting
	// to be resumed.
	//
	// It is a status of its own rather than a flag, because "stopped" and
	// "finished" have to be told apart: a stopped download has not produced a
	// file, and treating it as finished put it in the browser as a file that
	// was never written.
	StatusPaused
)
