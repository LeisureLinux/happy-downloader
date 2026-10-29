package formats

// Used to deser the formats in the -J output
type Metadata struct {
	Type      string     `json:"_type"`
	Formats   []Format   `json:"formats"`
	Best      Format     `json:"best"`
	Thumbnail string     `json:"thumbnail"`
	Title     string     `json:"title"`
	URL       string     `json:"url"`
	Entries   []Metadata `json:"entries"` // populated if url is playlist

	// Duration in seconds. The dialog uses it to size the download and to
	// decide whether an audio-only request makes sense.
	Duration float64 `json:"duration"`

	// Subtitles are the manually authored caption tracks, keyed by language
	// code. AutomaticCaptions are the generated ones, which are noisier but
	// always present. The dialog offers both because which one is wanted is a
	// per-video judgement.
	Subtitles         map[string]any `json:"subtitles"`
	AutomaticCaptions map[string]any `json:"automatic_captions"`

	// IsLive says whether the source is a livestream that is running now.
	//
	// yt-dlp works it out for every site it has an extractor for, so this
	// does not depend on a list of hosts kept in the application: a site
	// that starts being live tomorrow is handled without a change here.
	//
	// It is nil when the extractor does not say — most do not have the
	// field at all — and nil means "not known to be live", which is the
	// ordinary case for a video.
	IsLive *bool `json:"is_live,omitempty"`
}

func (m *Metadata) IsPlaylist() bool {
	return m.Type == "playlist"
}

// A skimmed yt-dlp format node
type Format struct {
	Format_id   string  `json:"format_id"`
	Format_note string  `json:"format_note"`
	FPS         float32 `json:"fps"`
	Resolution  string  `json:"resolution"`
	VCodec      string  `json:"vcodec"`
	ACodec      string  `json:"acodec"`
	Size        float64 `json:"filesize_approx"`
	Language    string  `json:"language"`
	// Ext is the container, e.g. mp4 / webm / m4a. It decides what
	// --merge-output-format can produce, since a container cannot always hold
	// the codecs the source streams use.
	Ext string `json:"ext"`
}
