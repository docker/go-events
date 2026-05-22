package events

import (
	"sync"
)

// ListenFunc is a type for listener functions must implement
type ListenFunc func(event Event)

// ListenerSink passes events through ListenFunc before draining through the
// next sink below
type ListenerSink struct {
	dst    Sink
	mu     sync.Mutex
	closed bool
	ListenFunc
}

// NewListenerSink returns a sink that will pass events to a listener
func NewListenerSink(dst Sink, fn ListenFunc) *ListenerSink {
	ls := &ListenerSink{
		dst:        dst,
		ListenFunc: fn,
		closed:     false,
	}
	return ls
}

// Write passes the event to a listener and then forwards to the next sink
func (ls *ListenerSink) Write(event Event) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.closed {
		return ErrSinkClosed
	}

	ls.ListenFunc(event)

	return ls.dst.Write(event)
}

// Close calls Close() on the sink below
func (ls *ListenerSink) Close() error {
	if ls.closed {
		return nil
	}

	// set closed flag
	ls.closed = true
	return ls.dst.Close()
}
