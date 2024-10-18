package events

import "time"

// Timeout provides an event sink that requires sent events to return in a
// specified amount of time or considers them to have failed.
type Timeout struct {
	dst     Sink
	timeout time.Duration
	closed  bool
}

// NewTimeout returns a new timeout to the provided dst sink.
func NewTimeout(dst Sink, timeout time.Duration) Sink {
	return &Timeout{dst: dst, timeout: timeout}
}

// Write an event to the timeout.
func (t *Timeout) Write(event Event) error {
	if t.closed {
		return ErrSinkClosed
	}

	errChan := make(chan error)
	go func(c chan<- error) {
		c <- t.dst.Write(event)
	}(errChan)

	timer := time.NewTimer(t.timeout)
	select {
	case err := <-errChan:
		timer.Stop()
		return err
	case <-timer.C:
		return ErrSinkTimeout
	}
}

// Close the timeout and allow no more events to pass through.
func (t *Timeout) Close() error {
	// TODO(stevvooe): Not all sinks should have Close.
	if t.closed {
		return nil
	}

	t.closed = true
	return t.dst.Close()
}