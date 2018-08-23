package conntrack

import (
	"sync"
	"time"
)

type EntryHandle struct {
	reason   string
	e        Entry
	priority priority
	i        *Instance
	expires  time.Time
	added    sync.Mutex
}

func (eh *EntryHandle) Done() {
	expvars.Add("entry handles done", 1)
	timeout := eh.timeout()
	eh.expires = time.Now().Add(timeout)
	if timeout <= 0 {
		eh.remove()
	} else {
		time.AfterFunc(eh.timeout(), eh.remove)
	}
}

func (eh *EntryHandle) Forget() {
	expvars.Add("entry handles forgotten", 1)
	eh.remove()
}

func (eh *EntryHandle) remove() {
	eh.i.remove(eh)
}

func (eh *EntryHandle) timeout() time.Duration {
	return eh.i.Timeout(eh.e)
}
