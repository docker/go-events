package events

import "fmt"

var (
	// ErrSinkClosed is returned if a write is issued to a sink that has been
	// closed. If encountered, the error should be considered terminal and
	// retries will not be successful.
	ErrSinkClosed = fmt.Errorf("events: sink closed")

	// ErrSinkTimeout is returned if a write is issued to a sink and it does
	// not return in the specified time. If encountered, the error may mean
	// that the sink is overloaded and retries may be successful.
	ErrSinkTimeout = fmt.Errorf("events: sink timeout")
)
