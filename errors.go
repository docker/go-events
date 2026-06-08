package events

import "fmt"

var (
	// ErrSinkClosed is returned if a write is issued to a sink that has been
	// closed. If encountered, the error should be considered terminal and
	// retries will not be successful.
	ErrSinkClosed = fmt.Errorf("events: sink closed")

	// ErrQueueFull is returned if a write is issued to a queue that does not
	// have enough space to store an additional event. If encountered, further
	// replies may be successful if any of the queue elements was consumed.
	ErrQueueFull = fmt.Errorf("events: queue full")
)
