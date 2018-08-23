package slices

import (
	"container/heap"
	"reflect"
	"sort"
)

// Sorts the slice in place. Returns sl for convenience.
func Sort(sl interface{}, less interface{}) interface{} {
	sorter := sorter{
		sl:   reflect.ValueOf(sl),
		less: reflect.ValueOf(less),
	}
	sort.Sort(&sorter)
	return sorter.sl.Interface()
}

// Creates a modifiable copy of a slice reference. Because you can't modify
// non-pointer types inside an interface{}.
func addressableSlice(slice interface{}) reflect.Value {
	v := reflect.ValueOf(slice)
	p := reflect.New(v.Type())
	p.Elem().Set(v)
	return p.Elem()
}

// Returns a "container/heap".Interface for the provided slice.
func HeapInterface(sl interface{}, less interface{}) heap.Interface {
	ret := &sorter{
		sl:   addressableSlice(sl),
		less: reflect.ValueOf(less),
	}
	heap.Init(ret)
	return ret
}
