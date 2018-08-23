package slices

import "reflect"

// sl []T, f is func(*T) bool.
func FilterInPlace(sl interface{}, f interface{}) {
	v := reflect.ValueOf(sl).Elem()
	j := 0
	for i := 0; i < v.Len(); i++ {
		e := v.Index(i)
		if reflect.ValueOf(f).Call([]reflect.Value{e.Addr()})[0].Bool() {
			v.Index(j).Set(e)
			j++
		}
	}
	v.SetLen(j)
}
