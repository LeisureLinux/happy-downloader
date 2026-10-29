package cookies

import (
	"os"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// Which cookie export is in use.
//
// There are two, and they are different things: the automatic one, which a
// timer replaces from a browser profile, and the one the user uploaded. They
// used to be resolved by a flag — "imported if there is one" — which meant the
// choice was made once, when the flag was set, and never looked at again.
//
// That is not the same question as "which one is current". The automatic
// export is replaced every ten minutes while its refresher runs, so it is
// normally the fresher of the two; the imported one is a moment in time. When
// the refresher is not running — which is the usual reason somebody imports a
// file by hand — the imported one is the fresher one, and it is the one that
// works.
//
// So the answer is the file that was written last, decided when the download
// is built rather than when the process started. A flag set at startup goes
// stale: the interface kept saying the imported export was in use while the
// downloads had fallen back to a day-old automatic one.

// The kinds of export, as reported to the interface.
const (
	KindImported  = "imported"
	KindAutomatic = "automatic"
)

// ActiveExport is the cookie file the downloads should use.
type ActiveExport struct {
	// Path is the file yt-dlp reads, empty when neither exists.
	Path string
	// Source is the file the export came from: the uploaded file, or the
	// browser export the timer replaces. It is what the interface shows,
	// because the converted copy is a detail of how the download is built.
	Source string
	// Kind is KindImported or KindAutomatic.
	Kind string
	// ModifiedAt is when the source was written.
	ModifiedAt time.Time
}

// Active picks the cookie file to use: the newer of the two.
//
// The imported export wins a tie, because it is the credential a person chose
// and the automatic one is the one that was already there.
func Active() ActiveExport {
	imported := importedExport()
	automatic := automaticExport()

	switch {
	case imported.Path == "" && automatic.Path == "":
		return ActiveExport{}
	case imported.Path == "":
		return automatic
	case automatic.Path == "":
		return imported
	case automatic.ModifiedAt.After(imported.ModifiedAt):
		return automatic
	default:
		return imported
	}
}

// importedExport is the uploaded export, when there is one.
func importedExport() ActiveExport {
	store := ImportedGlobal()
	if store == nil {
		return ActiveExport{}
	}

	cur := store.Current()
	if cur == nil || cur.Path == "" {
		return ActiveExport{}
	}

	export := statExport(cur.Path, KindImported, cur.ModifiedAt)
	export.Source = cur.Path

	return export
}

// automaticExport is the export the timer replaces, when one is configured.
//
// The age is the source's, not the converted copy's. The converted file is
// rewritten every time the arguments for a download are built, so its
// modification time is always "just now" — comparing against it made the
// automatic export win every time, and the import a person uploaded was never
// used again after the first download.
func automaticExport() ActiveExport {
	source := config.ResolvedCookieSource()
	if source == "" {
		return ActiveExport{}
	}

	converted := config.CookiesPath(source)
	if _, err := os.Stat(converted); err != nil {
		return ActiveExport{}
	}

	modified := time.Time{}
	if info, err := os.Stat(source); err == nil {
		modified = info.ModTime()
	} else if info, err := os.Stat(converted); err == nil {
		// The source is gone but a conversion exists: it is still what the
		// downloads use, and its age is the best answer available.
		modified = info.ModTime()
	}

	return ActiveExport{
		Path:       converted,
		Source:     source,
		Kind:       KindAutomatic,
		ModifiedAt: modified,
	}
}

// statExport asks the filesystem when a file was written.
//
// The fallback is for the imported export, whose store remembers the moment it
// was imported: that is the answer even if the file was touched afterwards.
func statExport(path, kind string, fallback time.Time) ActiveExport {
	info, err := os.Stat(path)
	if err != nil {
		return ActiveExport{}
	}

	modified := fallback
	if modified.IsZero() {
		modified = info.ModTime()
	}

	return ActiveExport{Path: path, Kind: kind, ModifiedAt: modified}
}
