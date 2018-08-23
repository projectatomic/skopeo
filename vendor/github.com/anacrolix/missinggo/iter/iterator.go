package iter

import "github.com/anacrolix/missinggo/slices"

type Iterator interface {
	// Advances to the next value. Returns false if there are no more values.
	// Must be called before the first value.
	Next() bool
	// Returns the current value. Should panic when the iterator is in an
	// invalid state.
	Value() interface{}
	// Ceases iteration prematurely. This should occur implicitly if Next
	// returns false.
	Stop()
}

func ToFunc(it Iterator) Func {
	return func(cb Callback) {
		defer it.Stop()
		for it.Next() {
			if !cb(it.Value()) {
				break
			}
		}
	}
}

type sliceIterator struct {
	slice []interface{}
	value interface{}
	ok    bool
}

func (me *sliceIterator) Next() bool {
	if len(me.slice) == 0 {
		return false
	}
	me.value = me.slice[0]
	me.slice = me.slice[1:]
	me.ok = true
	return true
}

func (me *sliceIterator) Value() interface{} {
	if !me.ok {
		panic("no value; call Next")
	}
	return me.value
}

func (me *sliceIterator) Stop() {}

func Slice(a []interface{}) Iterator {
	return &sliceIterator{
		slice: a,
	}
}

func StringIterator(a string) Iterator {
	return Slice(slices.ToEmptyInterface(a))
}

func ToSlice(f Func) (ret []interface{}) {
	f(func(v interface{}) bool {
		ret = append(ret, v)
		return true
	})
	return
}
