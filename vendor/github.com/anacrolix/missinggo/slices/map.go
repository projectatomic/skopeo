package slices

import "reflect"

type MapItem struct {
	Key, Elem interface{}
}

// Creates a []struct{Key K; Value V} for map[K]V.
func FromMap(m interface{}) (slice []MapItem) {
	mapValue := reflect.ValueOf(m)
	for _, key := range mapValue.MapKeys() {
		slice = append(slice, MapItem{key.Interface(), mapValue.MapIndex(key).Interface()})
	}
	return
}

// Returns all the elements []T, from m where m is map[K]T.
func FromMapElems(m interface{}) interface{} {
	inValue := reflect.ValueOf(m)
	outValue := reflect.MakeSlice(reflect.SliceOf(inValue.Type().Elem()), inValue.Len(), inValue.Len())
	for i, key := range inValue.MapKeys() {
		outValue.Index(i).Set(inValue.MapIndex(key))
	}
	return outValue.Interface()
}

// Returns all the elements []K, from m where m is map[K]T.
func FromMapKeys(m interface{}) interface{} {
	inValue := reflect.ValueOf(m)
	outValue := reflect.MakeSlice(reflect.SliceOf(inValue.Type().Key()), inValue.Len(), inValue.Len())
	for i, key := range inValue.MapKeys() {
		outValue.Index(i).Set(key)
	}
	return outValue.Interface()
}

// f: (T)T, input: []T, outout: []T
func Map(f, input interface{}) interface{} {
	inputValue := reflect.ValueOf(input)
	funcValue := reflect.ValueOf(f)
	_len := inputValue.Len()
	retValue := reflect.MakeSlice(reflect.TypeOf(input), _len, _len)
	for i := 0; i < _len; i++ {
		out := funcValue.Call([]reflect.Value{inputValue.Index(i)})
		retValue.Index(i).Set(out[0])
	}
	return retValue.Interface()
}
