// Package prioritybitmap implements a set of integers ordered by attached
// priorities.
package prioritybitmap

import (
	"sync"

	"github.com/anacrolix/missinggo/bitmap"
	"github.com/anacrolix/missinggo/iter"
	"github.com/anacrolix/missinggo/orderedmap"
)

var (
	bitSets = sync.Pool{
		New: func() interface{} {
			return make(map[int]struct{}, 1)
		},
	}
)

// Maintains set of ints ordered by priority.
type PriorityBitmap struct {
	mu sync.Mutex
	// From priority to singleton or set of bit indices.
	om orderedmap.OrderedMap
	// From bit index to priority
	priorities map[int]int
}

var _ bitmap.Interface = (*PriorityBitmap)(nil)

func (me *PriorityBitmap) Contains(bit int) bool {
	_, ok := me.priorities[bit]
	return ok
}

func (me *PriorityBitmap) Len() int {
	return len(me.priorities)
}

func (me *PriorityBitmap) Clear() {
	me.om = nil
	me.priorities = nil
}

func (me *PriorityBitmap) deleteBit(bit int) (priority int, ok bool) {
	priority, ok = me.priorities[bit]
	if !ok {
		return
	}
	switch v := me.om.Get(priority).(type) {
	case int:
		if v != bit {
			panic("invariant broken")
		}
	case map[int]struct{}:
		if _, ok := v[bit]; !ok {
			panic("invariant broken")
		}
		delete(v, bit)
		if len(v) != 0 {
			return
		}
		bitSets.Put(v)
	}
	me.om.Unset(priority)
	if me.om.Len() == 0 {
		me.om = nil
	}
	return
}

func bitLess(l, r interface{}) bool {
	return l.(int) < r.(int)
}

func (me *PriorityBitmap) lazyInit() {
	me.om = orderedmap.New(func(l, r interface{}) bool {
		return l.(int) < r.(int)
	})
	me.priorities = make(map[int]int)
}

// Returns true if the priority is changed, or the bit wasn't present.
func (me *PriorityBitmap) Set(bit int, priority int) bool {
	if p, ok := me.priorities[bit]; ok && p == priority {
		return false
	}
	if oldPriority, deleted := me.deleteBit(bit); deleted && oldPriority == priority {
		panic("should have already returned")
	}
	if me.priorities == nil {
		me.priorities = make(map[int]int)
	}
	me.priorities[bit] = priority
	if me.om == nil {
		me.om = orderedmap.New(bitLess)
	}
	_v, ok := me.om.GetOk(priority)
	if !ok {
		// No other bits with this priority, set it to a lone int.
		me.om.Set(priority, bit)
		return true
	}
	switch v := _v.(type) {
	case int:
		newV := bitSets.Get().(map[int]struct{})
		newV[v] = struct{}{}
		newV[bit] = struct{}{}
		me.om.Set(priority, newV)
	case map[int]struct{}:
		v[bit] = struct{}{}
	default:
		panic(v)
	}
	return true
}

func (me *PriorityBitmap) Remove(bit int) bool {
	me.mu.Lock()
	defer me.mu.Unlock()
	if _, ok := me.deleteBit(bit); !ok {
		return false
	}
	delete(me.priorities, bit)
	if len(me.priorities) == 0 {
		me.priorities = nil
	}
	if me.om != nil && me.om.Len() == 0 {
		me.om = nil
	}
	return true
}

func (me *PriorityBitmap) Iter(f iter.Callback) {
	me.IterTyped(func(i int) bool {
		return f(i)
	})
}

func (me *PriorityBitmap) IterTyped(_f func(i bitmap.BitIndex) bool) bool {
	me.mu.Lock()
	defer me.mu.Unlock()
	if me == nil || me.om == nil {
		return true
	}
	f := func(i int) bool {
		me.mu.Unlock()
		defer me.mu.Lock()
		return _f(i)
	}
	return iter.All(func(key interface{}) bool {
		value := me.om.Get(key)
		switch v := value.(type) {
		case int:
			return f(v)
		case map[int]struct{}:
			for i := range v {
				if !f(i) {
					return false
				}
			}
		}
		return true
	}, me.om.Iter)
}

func (me *PriorityBitmap) IsEmpty() bool {
	if me.om == nil {
		return true
	}
	return me.om.Len() == 0
}

// ok is false if the bit is not set.
func (me *PriorityBitmap) GetPriority(bit int) (prio int, ok bool) {
	prio, ok = me.priorities[bit]
	return
}
