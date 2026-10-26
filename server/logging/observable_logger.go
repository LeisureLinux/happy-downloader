package logging

import (
	"context"
	"sync"
)

// historySize is how many recent lines a newly connected client receives before
// the live stream starts. Opening the log panel used to show an empty box until
// the next line happened to be written, which made it useless for the question
// people actually open it with: what just happened.
const historySize = 200

/*
Logger implementation using the observable pattern.
Implements io.Writer interface.

The observable is an event source which drops everything unless there's
a subscriber connected.

A ring buffer keeps the most recent lines so a new subscriber starts with
context rather than an empty box. The buffer is deliberately memory-only: the
authoritative record is the file log, and this is the tail of it that happens
to be in memory.

The observer implementations are a http ServerSentEvents handler and a
websocket one in handler.go
*/
type ObservableLogger struct {
	logsChan chan []byte

	mu      sync.RWMutex
	history [][]byte
}

func NewObservableLogger() *ObservableLogger {
	return &ObservableLogger{
		logsChan: make(chan []byte, 100),
		history:  make([][]byte, 0, historySize),
	}
}

func (o *ObservableLogger) Write(p []byte) (n int, err error) {
	// Copy: the caller may reuse the slice after Write returns.
	line := make([]byte, len(p))
	copy(line, p)

	o.mu.Lock()
	o.history = append(o.history, line)
	if len(o.history) > historySize {
		o.history = o.history[len(o.history)-historySize:]
	}
	o.mu.Unlock()

	select {
	case o.logsChan <- line:
		return len(p), nil
	default:
		// No subscriber, or the buffer is full. The line is already in the
		// history, so dropping it here loses nothing a reader can observe.
		return len(p), nil
	}
}

// History returns a copy of the buffered lines, oldest first.
func (o *ObservableLogger) History() [][]byte {
	o.mu.RLock()
	defer o.mu.RUnlock()

	out := make([][]byte, len(o.history))
	copy(out, o.history)
	return out
}

func (o *ObservableLogger) Observe(ctx context.Context) <-chan string {
	logs := make(chan string)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case logLine := <-o.logsChan:
				select {
				case logs <- string(logLine):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return logs
}
