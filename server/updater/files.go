package updater

import "os"

// The small file operations the updater needs, in one place.
//
// They exist because os.Rename and os.Create do not do quite what an update
// needs: the new binary has to be executable before it is put in place, and
// the replacement has to be atomic so that a download starting at that moment
// finds either the old binary or the new one.

// createExecutable creates a file that can be run.
func createExecutable(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// replaceFile puts one file in place of another, atomically.
//
// The rename is what makes it atomic: the destination is never a half-written
// file, and a reader either sees the old file or the new one.
func replaceFile(from, to string) error {
	return os.Rename(from, to)
}

// removeQuietly deletes a file, ignoring the failure.
//
// It is used on the cleanup path, where the file is already known to be
// unwanted and the error would say nothing useful.
func removeQuietly(path string) {
	os.Remove(path)
}
