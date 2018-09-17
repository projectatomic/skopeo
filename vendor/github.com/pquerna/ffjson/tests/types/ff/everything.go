/**
 *  Copyright 2014 Paul Querna
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package ff

import (
	"regexp"
	"runtime"
	"strconv"

	"github.com/foo/vendored"
)

// ExpectedSomethingValue maybe expects something of value
var ExpectedSomethingValue int8

// GoLangVersionPre16 indicates if golang before 1.6
var GoLangVersionPre16 bool

func init() {
	// since go1.6 reflect package changed behaivour:
	//
	// --------
	// https://tip.golang.org/doc/go1.6
	//
	// The reflect package has resolved a long-standing incompatibility between
	// the gc and gccgo toolchains regarding embedded unexported struct types
	// containing exported fields. Code that walks data structures using
	// reflection, especially to implement serialization in the spirit of the
	// encoding/json and encoding/xml packages, may need to be updated.
	//
	// The problem arises when using reflection to walk through an embedded
	// unexported struct-typed field into an exported field of that struct. In
	// this case, reflect had incorrectly reported the embedded field as exported,
	// by returning an empty Field.PkgPath. Now it correctly reports the field as
	// unexported but ignores that fact when evaluating access to exported fields
	// contained within the struct.
	//
	// Updating: Typically, code that previously walked over structs and used
	//
	// f.PkgPath != ""
	// to exclude inaccessible fields should now use
	//
	// f.PkgPath != "" && !f.Anonymous
	// For example, see the changes to the implementations of encoding/json and
	// encoding/xml.
	//
	// --------
	//
	// I didn't find better option to get Go's version rather then parsing
	// runtime.Version(). Godoc say that Version() can return multiple things:
	//
	// Version returns the Go tree's version string. It is either the commit
	// hash and date at the time of the build or, when possible, a release tag
	// like "go1.3".
	//
	// So, I'll assumes that if Version() returns not a release tag, running
	// version is younger then 1.5. Patches welcome :-)

	versionRegexp := regexp.MustCompile("^go[0-9]+\\.([0-9]+)")
	if res := versionRegexp.FindStringSubmatch(runtime.Version()); len(res) > 1 {
		if i, _ := strconv.Atoi(res[1]); i < 6 {
			// pre go1.6
			GoLangVersionPre16 = true
			ExpectedSomethingValue = 99
		}
	}
}

// SweetInterface is a sweet interface
type SweetInterface interface {
	Cats() int
}

// Cats they allways fallback on their legs
type Cats struct {
	FieldOnCats int
}

// Cats initialize a cat
func (c *Cats) Cats() int {
	return 42
}

// Embed structure
type Embed struct {
	SuperBool bool
}

// Everything a bit of everything... take care what yy-ou which for
type Everything struct {
	Embed
	Bool             bool
	Int              int
	Int8             int8
	Int16            int16
	Int32            int32
	Int64            int64
	Uint             uint
	Uint8            uint8
	Uint16           uint16
	Uint32           uint32
	Uint64           uint64
	Uintptr          uintptr
	Float32          float32
	Float64          float64
	Array            [2]int
	Slice            []int
	SlicePointer     *[]string
	Map              map[string]int
	String           string
	StringPointer    *string
	Int64Pointer     *int64
	FooStruct        *Foo
	MySweetInterface SweetInterface
	MapMap           map[string]map[string]string
	MapArraySlice    map[string][3][]int
	nonexported
}

type nonexported struct {
	Something int8
}

// Foo a foo's structure (it's a bar !?!)
type Foo struct {
	Bar int
	Baz vendored.Foo
}

// NewEverything kind of renew the world
func NewEverything(e *Everything) {
	e.SuperBool = true
	e.Bool = true
	e.Int = 1
	e.Int8 = 2
	e.Int16 = 3
	e.Int32 = -4
	e.Int64 = 2 ^ 59
	e.Uint = 100
	e.Uint8 = 101
	e.Uint16 = 102
	e.Uint64 = 103
	e.Uintptr = 104
	e.Float32 = 3.14
	e.Float64 = 3.15
	e.Array = [2]int{11, 12}
	e.Slice = []int{1, 2, 3}
	e.SlicePointer = &[]string{"a", "b"}
	e.Map = map[string]int{
		"foo": 1,
		"bar": 2,
	}
	e.String = "snowman->☃"
	e.FooStruct = &Foo{Bar: 1, Baz: vendored.Foo{A: "a", B: 1}}
	e.Something = ExpectedSomethingValue
	e.MySweetInterface = &Cats{}
	e.MapMap = map[string]map[string]string{
		"a": map[string]string{"b": "2", "c": "3", "d": "4"},
		"e": map[string]string{},
		"f": map[string]string{"g": "9"},
	}
	e.MapArraySlice = map[string][3][]int{
		"a": [3][]int{
			0: []int{1, 2, 3},
			1: []int{},
			2: []int{4},
		},
	}
}
