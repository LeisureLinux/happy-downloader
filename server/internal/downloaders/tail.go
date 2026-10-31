package downloaders

import (
	"bytes"
	"sync"
)

// tail keeps the last few lines written to it.
//
// It is for the case where the useful part of a failure is the end of what the
// program said, and the rest of it has already gone somewhere else — the
// interface's log panel, which nobody is reading by the time the failure is
// reported.
type tail struct {
	mu    sync.Mutex
	lines []string
	keep  int
	part  bytes.Buffer
}

func newTail(keep int) *tail {
	return &tail{keep: keep}
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.part.Write(p)

	for {
		line, err := t.part.ReadString('\n')
		if err != nil {
			// No newline yet: put the partial line back and wait for more.
			t.part.Reset()
			t.part.WriteString(line)

			break
		}

		t.lines = append(t.lines, trimNewline(line))

		if len(t.lines) > t.keep {
			t.lines = t.lines[len(t.lines)-t.keep:]
		}
	}

	return len(p), nil
}

// String is what was said last, one line per line, skipping the blank ones.
func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	lines := t.lines
	if rest := trimNewline(t.part.String()); rest != "" {
		lines = append(append([]string{}, lines...), rest)
	}

	var b bytes.Buffer

	for _, line := range lines {
		if line == "" {
			continue
		}

		if b.Len() > 0 {
			b.WriteString("; ")
		}

		b.WriteString(line)
	}

	return b.String()
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}

	return s
}
