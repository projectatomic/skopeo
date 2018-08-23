package perf

import (
	"log"
	"runtime"
	"time"
)

type Timer struct {
	started time.Time
	log     bool
	name    string
	marked  bool
}

func NewTimer(opts ...timerOpt) (t *Timer) {
	t = &Timer{
		started: time.Now(),
	}
	for _, o := range opts {
		o(t)
	}
	if t.log && t.name != "" {
		log.Printf("starting timer %q", t.name)
	}
	runtime.SetFinalizer(t, func(t *Timer) {
		if t.marked {
			return
		}
		log.Printf("timer %#v was never marked", t)
	})
	return
}

type timerOpt func(*Timer)

func Log(t *Timer) {
	t.log = true
}

func Name(name string) func(*Timer) {
	return func(t *Timer) {
		t.name = name
	}
}

func (t *Timer) Mark(events ...string) time.Duration {
	d := time.Since(t.started)
	if len(events) == 0 {
		if t.name == "" {
			panic("no name or events specified")
		}
		t.addDuration(t.name, d)
	} else {
		for _, e := range events {
			if t.name != "" {
				e = t.name + "/" + e
			}
			t.addDuration(e, d)
		}
	}
	return d
}

func (t *Timer) MarkOk(ok bool) {
	if ok {
		t.Mark("ok")
	} else {
		t.Mark("not ok")
	}
}

func (t *Timer) MarkErr(err error) {
	if err == nil {
		t.Mark("success")
	} else {
		t.Mark("error")
	}
}

func (t *Timer) addDuration(desc string, d time.Duration) {
	t.marked = true
	mu.RLock()
	e := events[desc]
	mu.RUnlock()
	if e == nil {
		mu.Lock()
		e = events[desc]
		if e == nil {
			e = new(Event)
			e.Init()
			events[desc] = e
		}
		mu.Unlock()
	}
	e.Add(d)
	if t.log {
		if t.name != "" {
			log.Printf("timer %q got event %q after %s", t.name, desc, d)
		} else {
			log.Printf("marking event %q after %s", desc, d)
		}
	}
}
