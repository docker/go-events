package events

import "fmt"

var (
	// ErrSinkClosed is returned if a write is issued to a sink that has been
	// closed. If encountered, the error should be considered terminal and
	// retries will not be successful.
	ErrSinkClosed = fmt.Errorf("events: sink closed")
	// ErrNoSinks is returned if a write is issued to a round-robin sink without
	// destiny. If encountered, destiny sinks should be added before calling
	// Write again.
	ErrNoSinks = fmt.Errorf("events: no destiny sink")
)
