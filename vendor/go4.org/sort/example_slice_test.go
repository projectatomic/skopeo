// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sort_test

import (
	"fmt"

	"go4.org/sort"
)

func Example() {
	people := []Person{
		{Name: "Bob", Age: 31},
		{Name: "John", Age: 42},
		{Name: "Michael", Age: 17},
		{Name: "Jenny", Age: 26},
	}

	fmt.Println(people)
	sort.Slice(people, func(i, j int) bool { return people[i].Age < people[j].Age })
	fmt.Println(people)

	// Output:
	// [Bob: 31 John: 42 Michael: 17 Jenny: 26]
	// [Michael: 17 Jenny: 26 Bob: 31 John: 42]
}

func ExampleSlice() {
	people := []Person{
		{Name: "Bob", Age: 31},
		{Name: "John", Age: 42},
		{Name: "Michael", Age: 17},
		{Name: "Jenny", Age: 26},
	}

	fmt.Println(people)
	sort.Slice(people, func(i, j int) bool { return people[i].Age < people[j].Age })
	fmt.Println(people)

	// Output:
	// [Bob: 31 John: 42 Michael: 17 Jenny: 26]
	// [Michael: 17 Jenny: 26 Bob: 31 John: 42]
}
