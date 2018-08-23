About
=====

This is a library implementing skip lists for the Go programming
language (http://golang.org/).

Skip lists are a data structure that can be used in place of
balanced trees. Skip lists use probabilistic balancing rather than
strictly enforced balancing and as a result the algorithms for
insertion and deletion in skip lists are much simpler and
significantly faster than equivalent algorithms for balanced trees.

Skip lists were first described in
[Pugh, William (June 1990)](ftp://ftp.cs.umd.edu/pub/skipLists/skiplists.pdf). "Skip
lists: a probabilistic alternative to balanced trees". Communications
of the ACM 33 (6): 668–676

[![Build Status](https://travis-ci.org/ryszard/goskiplist.png?branch=master)](https://travis-ci.org/ryszard/goskiplist)

Installing
==========

    $ go get github.com/ryszard/goskiplist/skiplist

Example
=======

```go
package main

import (
	"fmt"
	"github.com/ryszard/goskiplist/skiplist"
)

func main() {
	s := skiplist.NewIntMap()
	s.Set(7, "seven")
	s.Set(1, "one")
	s.Set(0, "zero")
	s.Set(5, "five")
	s.Set(9, "nine")
	s.Set(10, "ten")
	s.Set(3, "three")

	firstValue, ok := s.Get(0)
	if ok {
		fmt.Println(firstValue)
	}
	// prints:
	//  zero

	s.Delete(7)

	secondValue, ok := s.Get(7)
	if ok {
		fmt.Println(secondValue)
	}
	// prints: nothing.

	s.Set(9, "niner")

	// Iterate through all the elements, in order.
	unboundIterator := s.Iterator()
	for unboundIterator.Next() {
		fmt.Printf("%d: %s\n", unboundIterator.Key(), unboundIterator.Value())
	}
	// prints:
	//  0: zero
	//  1: one
	//  3: three
	//  5: five
	//  9: niner
	//  10: ten

	for unboundIterator.Previous() {
		fmt.Printf("%d: %s\n", unboundIterator.Key(), unboundIterator.Value())
	}
	//  9: niner
	//  5: five
	//  3: three
	//  1: one
	//  0: zero

	boundIterator := s.Range(3, 10)
	// Iterate only through elements in some range.
	for boundIterator.Next() {
		fmt.Printf("%d: %s\n", boundIterator.Key(), boundIterator.Value())
	}
	// prints:
	//  3: three
	//  5: five
	//  9: niner

	for boundIterator.Previous() {
		fmt.Printf("%d: %s\n", boundIterator.Key(), boundIterator.Value())
	}
	// prints:
	//  5: five
	//  3: three

	var iterator skiplist.Iterator

	iterator = s.Seek(3)
	fmt.Printf("%d: %s\n", iterator.Key(), iterator.Value())
	// prints:
	//  3: three

	iterator = s.Seek(2)
	fmt.Printf("%d: %s\n", iterator.Key(), iterator.Value())
	// prints:
	//  3: three

  iterator = s.SeekToFirst()
  fmt.Printf("%d: %s\n", iterator.Key(), iterator.Value())
  // prints:
  //  0: zero

  iterator = s.SeekToLast()
  fmt.Printf("%d: %s\n", iterator.Key(), iterator.Value())
  // prints:
  //  10: ten

  // SkipList can also reduce subsequent forward seeking costs by reusing the
  // same iterator:

  iterator = s.Seek(3)
	fmt.Printf("%d: %s\n", iterator.Key(), iterator.Value())
	// prints:
	//  3: three

  iterator.Seek(5)
	fmt.Printf("%d: %s\n", iterator.Key(), iterator.Value())
	// prints:
	//  5: five
}
```

Full documentation
==================

Read it [online](http://godoc.org/github.com/ryszard/goskiplist/skiplist) or run

    $ go doc github.com/ryszard/goskiplist/skiplist

Other implementations
=====================

This list is probably incomplete.


  * https://github.com/huandu/skiplist
  * https://bitbucket.org/taruti/go-skip/src
  * http://code.google.com/p/leveldb-go/source/browse/leveldb/memdb/memdb.go
  (part of [leveldb-go](http://code.google.com/p/leveldb-go/))
