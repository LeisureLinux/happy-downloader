package logging

import (
	"errors"
	"strings"
	"testing"
)

// A destination that cannot be written to must not silence the others.
//
// This is the bug that made the log panel empty on Windows: the terminal was
// first in the chain, the terminal did not exist, and every line stopped there.
type failingSink struct {
	attempts int
}

func (f *failingSink) Write(p []byte) (int, error) {
	f.attempts++
	return 0, errors.New("The handle is invalid.")
}

type recordingSink struct {
	lines []string
}

func (r *recordingSink) Write(p []byte) (int, error) {
	r.lines = append(r.lines, string(p))
	return len(p), nil
}

func TestFanoutKeepsGoingAfterAFailure(t *testing.T) {
	bad := &failingSink{}
	good := &recordingSink{}

	w := FanoutWriter(bad, good)

	if _, err := w.Write([]byte("a log line\n")); err == nil {
		t.Fatal("the write should report the failure it met")
	}

	if len(good.lines) != 1 {
		t.Fatalf("the working destination got %d lines, want 1", len(good.lines))
	}

	if bad.attempts != 1 {
		t.Fatalf("the broken destination was tried %d times, want 1", bad.attempts)
	}
}

func TestFanoutCountsTheWholeLineEvenWhenADestinationFails(t *testing.T) {
	w := FanoutWriter(&failingSink{})

	n, err := w.Write([]byte("hello"))

	if n != len("hello") {
		t.Fatalf("wrote %d bytes, want %d: a short count makes the caller write it again", n, len("hello"))
	}

	if err == nil {
		t.Fatal("the error should still be reported")
	}
}

func TestFanoutReachesEveryDestination(t *testing.T) {
	first := &recordingSink{}
	second := &recordingSink{}

	w := FanoutWriter(first, second)

	if _, err := w.Write([]byte("line\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for name, sink := range map[string]*recordingSink{"first": first, "second": second} {
		if got := strings.Join(sink.lines, ""); got != "line\n" {
			t.Fatalf("%s got %q, want %q", name, got, "line\n")
		}
	}
}

func TestFanoutSkipsNilWriters(t *testing.T) {
	only := &recordingSink{}

	w := FanoutWriter(nil, only, nil)

	if _, err := w.Write([]byte("line\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(only.lines) != 1 {
		t.Fatalf("the one real destination got %d lines, want 1", len(only.lines))
	}
}
