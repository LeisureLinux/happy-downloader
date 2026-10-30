package logging

import "io"

// Fanout writes to every destination, whatever the others do.
//
// The application logs to more than one place at once: the terminal, the log
// panel in the interface, and the file. io.MultiWriter looks like it does the
// same thing and does not — it stops at the first writer that returns an
// error. One destination that cannot be written to therefore silences all the
// others.
//
// That is not hypothetical. The Windows build has no console, so its standard
// output is an invalid handle, and every write to it fails. The log panel was
// empty because the terminal it was queued behind could not be written to, and
// the terminal it was queued behind was not there.
//
// So each destination is written to on its own. A destination that fails is
// skipped, and the last error is returned so that a caller that cares can see
// it — slog ignores it either way.
type Fanout struct {
	writers []io.Writer
}

// FanoutWriter returns a writer that writes to all of them.
func FanoutWriter(writers ...io.Writer) *Fanout {
	kept := make([]io.Writer, 0, len(writers))

	for _, w := range writers {
		if w != nil {
			kept = append(kept, w)
		}
	}

	return &Fanout{writers: kept}
}

func (f *Fanout) Write(p []byte) (int, error) {
	var firstErr error

	// One destination refusing is not a reason for the others to lose the
	// line, which is the whole point of this type.
	for _, w := range f.writers {
		if _, err := w.Write(p); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// The count is always the full length: partial delivery is still delivery,
	// and a short count would make the caller try to write the rest again.
	return len(p), firstErr
}
