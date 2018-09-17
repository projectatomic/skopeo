// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflectutil

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

func TestSwapper(t *testing.T) {
	type I int
	var a, b, c I
	type pair struct {
		x, y int
	}
	type pairPtr struct {
		x, y int
		p    *I
	}
	type S string

	tests := []struct {
		in   interface{}
		i, j int
		want interface{}
	}{
		{
			in:   []int{1, 20, 300},
			i:    0,
			j:    2,
			want: []int{300, 20, 1},
		},
		{
			in:   []uintptr{1, 20, 300},
			i:    0,
			j:    2,
			want: []uintptr{300, 20, 1},
		},
		{
			in:   []int16{1, 20, 300},
			i:    0,
			j:    2,
			want: []int16{300, 20, 1},
		},
		{
			in:   []int8{1, 20, 100},
			i:    0,
			j:    2,
			want: []int8{100, 20, 1},
		},
		{
			in:   []*I{&a, &b, &c},
			i:    0,
			j:    2,
			want: []*I{&c, &b, &a},
		},
		{
			in:   []string{"eric", "sergey", "larry"},
			i:    0,
			j:    2,
			want: []string{"larry", "sergey", "eric"},
		},
		{
			in:   []S{"eric", "sergey", "larry"},
			i:    0,
			j:    2,
			want: []S{"larry", "sergey", "eric"},
		},
		{
			in:   []pair{{1, 2}, {3, 4}, {5, 6}},
			i:    0,
			j:    2,
			want: []pair{{5, 6}, {3, 4}, {1, 2}},
		},
		{
			in:   []pairPtr{{1, 2, &a}, {3, 4, &b}, {5, 6, &c}},
			i:    0,
			j:    2,
			want: []pairPtr{{5, 6, &c}, {3, 4, &b}, {1, 2, &a}},
		},
	}
	for i, tt := range tests {
		inStr := fmt.Sprint(tt.in)
		Swapper(tt.in)(tt.i, tt.j)
		if !reflect.DeepEqual(tt.in, tt.want) {
			t.Errorf("%d. swapping %v and %v of %v = %v; want %v", i, tt.i, tt.j, inStr, tt.in, tt.want)
		}
	}
}

func BenchmarkSwap(b *testing.B) {
	const N = 1024
	strs := make([]string, N)
	for i := range strs {
		strs[i] = strconv.Itoa(i)
	}
	swap := Swapper(strs)

	b.ResetTimer()
	i, j := 0, 1
	for n := 0; n < b.N; n++ {
		i = (i + 1) % N
		j = (j + 2) % N
		swap(i, j)
	}
}
