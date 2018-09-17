// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflectutil

import (
	"reflect"
	"testing"
)

func TestHasPointers(t *testing.T) {
	tests := []struct {
		val  interface{}
		want bool
	}{
		{false, false},
		{int(1), false},
		{int8(1), false},
		{int16(1), false},
		{int32(1), false},
		{int64(1), false},
		{uint(1), false},
		{uint8(1), false},
		{uint16(1), false},
		{uint32(1), false},
		{uint64(1), false},
		{uintptr(1), false},
		{float32(1.0), false},
		{float64(1.0), false},
		{complex64(1.0i), false},
		{complex128(1.0i), false},

		{[...]int{1, 2}, false},
		{[...]*int{nil, nil}, true},

		{make(chan bool), true},

		{TestHasPointers, true},

		{map[string]string{"foo": "bar"}, true},

		{new(int), true},

		{[]int{1, 2}, true},

		{"foo", true},

		{struct{}{}, false},
		{struct{ int }{0}, false},
		{struct {
			a int
			b bool
		}{0, false}, false},
		{struct {
			a int
			b string
		}{0, ""}, true},
		{struct{ *int }{nil}, true},
		{struct {
			a *int
			b int
		}{nil, 0}, true},
	}
	for i, tt := range tests {
		got := hasPointers(reflect.TypeOf(tt.val))
		if got != tt.want {
			t.Errorf("%d. hasPointers(%T) = %v; want %v", i, tt.val, got, tt.want)
		}
	}
}
