package missinggo

import "sync"

type SynchronizedEvent struct {
	mu sync.Mutex
	e  Event
}

func (me *SynchronizedEvent) Set() {
	me.mu.Lock()
	me.e.Set()
	me.mu.Unlock()
}

func (me *SynchronizedEvent) Clear() {
	me.mu.Lock()
	me.e.Clear()
	me.mu.Unlock()
}

func (me *SynchronizedEvent) C() <-chan struct{} {
	me.mu.Lock()
	defer me.mu.Unlock()
	return me.e.C()
}
