package perf

import (
	"math"
	"sync"
	"time"
)

type Event struct {
	Mu    sync.RWMutex
	Count int64
	Total time.Duration
	Min   time.Duration
	Max   time.Duration
}

func (e *Event) Add(t time.Duration) {
	e.Mu.Lock()
	defer e.Mu.Unlock()
	if t > e.Max {
		e.Max = t
	}
	if t < e.Min {
		e.Min = t
	}
	e.Count++
	e.Total += t
}

func (e *Event) MeanTime() time.Duration {
	e.Mu.RLock()
	defer e.Mu.RUnlock()
	return e.Total / time.Duration(e.Count)
}

func (e *Event) Init() {
	e.Min = math.MaxInt64
}
