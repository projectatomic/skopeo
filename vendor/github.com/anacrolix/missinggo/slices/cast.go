package slices

import (
	"reflect"

	"github.com/bradfitz/iter"
)

// Returns a copy of all the elements of slice []T as a slice of interface{}.
func ToEmptyInterface(slice interface{}) (ret []interface{}) {
	v := reflect.ValueOf(slice)
	l := v.Len()
	ret = make([]interface{}, 0, l)
	for i := range iter.N(v.Len()) {
		ret = append(ret, v.Index(i).Interface())
	}
	return
}

// Makes and sets a slice at *ptrTo, and type asserts all the elements from
// from to it.
func MakeInto(ptrTo interface{}, from interface{}) {
	fromSliceValue := reflect.ValueOf(from)
	fromLen := fromSliceValue.Len()
	if fromLen == 0 {
		return
	}
	// Deref the pointer to slice.
	slicePtrValue := reflect.ValueOf(ptrTo)
	if slicePtrValue.Kind() != reflect.Ptr {
		panic("destination is not a pointer")
	}
	destSliceValue := slicePtrValue.Elem()
	// The type of the elements of the destination slice.
	destSliceElemType := destSliceValue.Type().Elem()
	destSliceValue.Set(reflect.MakeSlice(destSliceValue.Type(), fromLen, fromLen))
	for i := range iter.N(fromSliceValue.Len()) {
		// The value inside the interface in the slice element.
		itemValue := fromSliceValue.Index(i)
		if itemValue.Kind() == reflect.Interface {
			itemValue = itemValue.Elem()
		}
		convertedItem := itemValue.Convert(destSliceElemType)
		destSliceValue.Index(i).Set(convertedItem)
	}
}
