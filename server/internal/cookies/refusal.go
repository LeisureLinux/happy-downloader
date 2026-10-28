package cookies

import (
	"strings"
	"sync"
)

// Detecting that a cookie was refused.
//
// yt-dlp does not say "your cookie is bad". It says "The provided YouTube
// account cookies are no longer valid", or "Sign in to confirm you're not a
// bot", or — the quiet one — it succeeds with a guest session and the
// download produces a file that cannot be watched.
//
// The wording is matched on the part that does not change, not on the whole
// sentence. yt-dlp's exact text is:
//
//	"The provided YouTube account cookies are no longer valid. They have
//	 likely been rotated in the browser as a security measure. ..."
//
// and matching the whole sentence missed it, because the site name is in
// the middle of it — "YouTube account cookies", not "cookies". The extractor
// for another site says the same thing with its own name in it.
//
// So the patterns are the parts that are always there.

// The phrases that mean the credential was refused.
//
// Every one of them is matched against the lower-cased message, so the
// capitalisation of yt-dlp's output does not matter.
var refusalPhrases = []string{
	// The cookie was rotated in the browser. The site's name is in the
	// middle of the sentence, so only the part around it is matched.
	"account cookies are no longer valid",
	"cookies are no longer valid",

	// The bot wall: the request was made as a guest.
	"sign in to confirm you're not a bot",
	"sign in to confirm you’re not a bot",
	"sign in to confirm that you're not a bot",

	// The file could not be read as a cookie jar at all.
	"unable to extract any cookies",
	"cookies file must be netscape formatted",

	// The request itself was refused.
	"http error 401",
	"http error 403: forbidden",
	"login required",
}

// LooksRefused reports whether a message means the credential was refused.
//
// The bot wall and the rotated cookie are the same thing seen from two
// sides, and both mean the same thing here: the credential this download
// used is not the credential the site accepts.
func LooksRefused(message string) bool {
	lower := strings.ToLower(message)

	for _, phrase := range refusalPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}

	return false
}

// The counter of refusals, and who to tell about them.
//
// It is a package-level variable rather than a field on the store because the
// thing that notices a refusal is the downloader, and the thing that owns the
// file is the store. The hook is how they are connected, and the server sets
// it once at startup.
var (
	failureMu sync.RWMutex
	// onFailure is called when a download using the imported export was
	// refused.
	onFailure func()
)

// SetFailureHook registers the function to call when a download using the
// imported export is refused.
//
// The hook is what removes the file: after the second refusal the exported
// credential is not working, and the automatic export underneath is a better
// answer than a credential the site has rejected.
func SetFailureHook(fn func()) {
	failureMu.Lock()
	onFailure = fn
	failureMu.Unlock()
}

// ReportRefusal tells the store that the imported export was refused.
//
// It is called from wherever yt-dlp's output says the credential was
// rejected — the downloader and the caption fetch both. Only the imported
// export counts: the automatic export is replaced by the refresh timer every
// ten minutes, so a refusal of that one is a transient condition rather than
// a reason to delete a file.
func ReportRefusal() {
	failureMu.RLock()
	fn := onFailure
	failureMu.RUnlock()

	if fn != nil {
		fn()
	}
}
