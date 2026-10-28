package downloaders

import "sync"

// The cookie export the user imported through the interface.
//
// The downloader needs one thing from it: the path. The rest — how it is
// doing, whether it has been refused — belongs to the cookie store, which is
// in another package and must not be imported from here, or the two would
// depend on each other.
//
// So the store publishes the path here, and this is the only way the
// downloader learns about it.

// importedCookie holds the path of the imported export.
var importedCookie struct {
	sync.RWMutex
	path string
}

// SetImportedCookiePath publishes the path of the imported cookie export.
//
// An empty path means there is none, and the automatic export is used.
func SetImportedCookiePath(path string) {
	importedCookie.Lock()
	importedCookie.path = path
	importedCookie.Unlock()
}

// ImportedCookiePath returns the path of the imported cookie export, or an
// empty string when there is none.
func ImportedCookiePath() string {
	importedCookie.RLock()
	defer importedCookie.RUnlock()

	return importedCookie.path
}
