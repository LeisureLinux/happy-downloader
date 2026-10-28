package cookies

import (
	"strings"
	"sync"
)

// Detecting that a cookie was refused.
//
// yt-dlp does not say "your cookie is bad". It says "Sign in to confirm you're
// not a bot", or "cookies are no longer valid", or — the quiet one — it
// succeeds with a guest session and the download produces a file that cannot
// be watched.
//
// So the detection is on the error text, and the failure counter is what
// turns a suspicion into a decision: after two refusals the imported export
// is removed and the automatic one takes over again.

// The phrases yt-dlp uses when a credential is refused.
var refusalPhrases = []string{
	"sign in to confirm you're not a bot",
	"sign in to confirm you’re not a bot",
	"cookies are no longer valid",
	"the cookies are not valid",
	"unable to extract any cookies",
	"http error 403: forbidden",
	"login required",
}

// LooksRefused reports whether a message means the credential was refused.
//
// The bot wall is checked first because it is the one that matters: it is
// what a stale or wrong cookie looks like from the outside, and it is what
// the automatic export is there to prevent.
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
// It is called from the downloader when yt-dlp's output says the credential
// was rejected. Only the imported export counts: the automatic export is
// replaced by the refresh timer every ten minutes, so a refusal of that one
// is a transient condition rather than a reason to delete a file.
func ReportRefusal() {
	failureMu.RLock()
	fn := onFailure
	failureMu.RUnlock()

	if fn != nil {
		fn()
	}
}
