package missinggo

import "sync"

// Events are boolean flags that provide a channel that's closed when true.
// This could go in the sync package, but that's more of a debug wrapper on
// the standard library sync.
type Event struct {
	ch     chan struct{}
	closed bool
}

func (me *Event) LockedChan(lock sync.Locker) <-chan struct{} {
	lock.Lock()
	ch := me.C()
	lock.Unlock()
	return ch
}

// Returns a chan that is closed when the event is true.
func (me *Event) C() <-chan struct{} {
	if me.ch == nil {
		me.ch = make(chan struct{})
	}
	return me.ch
}

// TODO: Merge into Set.
func (me *Event) Clear() {
	if me.closed {
		me.ch = nil
		me.closed = false
	}
}

// Set the event to true/on.
func (me *Event) Set() (first bool) {
	if me.closed {
		return false
	}
	if me.ch == nil {
		me.ch = make(chan struct{})
	}
	close(me.ch)
	me.closed = true
	return true
}

// TODO: Change to Get.
func (me *Event) IsSet() bool {
	return me.closed
}

func (me *Event) Wait() {
	<-me.C()
}

// TODO: Merge into Set.
func (me *Event) SetBool(b bool) {
	if b {
		me.Set()
	} else {
		me.Clear()
	}
}
