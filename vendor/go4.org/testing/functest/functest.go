/*
Copyright 2016 The go4.org Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package functest contains utilities to ease writing table-driven
// tests for pure functions and method.
//
// Example:
//
//	func square(v int) int { return v * v }
//
//	func TestFunc(t *testing.T) {
//		f := functest.New(square)
//		f.Test(t,
//			f.In(0).Want(0),
//			f.In(1).Want(1),
//			f.In(2).Want(4),
//			f.In(3).Want(9),
//		)
//	}
//
// It can test whether things panic:
//
//	f := functest.New(condPanic)
//	f.Test(t,
//		f.In(false, nil),
//		f.In(true, "boom").Check(func(res functest.Result) error {
//			if res.Panic != "boom" {
//				return fmt.Errorf("panic = %v; want boom", res.Panic)
//			}
//			return nil
//		}),
//		f.In(true, nil).Check(func(res functest.Result) error {
//			if res.Panic != nil || res.Paniked {
//				return fmt.Errorf("expected panic with nil value, got: %+v", res)
//			}
//			return nil
//		}),
//	)
//
// If a test fails, functest does its best to format a useful error message. You can also
// name test cases:
//
//		f := functest.New(square)
//		f.Test(t,
//			f.In(0).Want(0),
//			f.In(1).Want(111),
//			f.In(2).Want(4),
//			f.Case("three").In(3).Want(999),
//		)
//
// Which would fail like:
//
//	--- FAIL: TestSquare (0.00s)
//	functest.go:304: square(1) = 1; want 111
//	functest.go:304: three: square(3) = 9; want 999
//	FAIL
//
package functest

import (
	"bytes"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Func is a wrapper around a func to test.
// It must be created with New.
type Func struct {
	// Name is the name of the function to use in error messages.
	// In most cases it is initialized by New, unless the function
	// being tested is an anonymous function.
	Name string

	f  interface{}   // the func
	fv reflect.Value // of f
}

var removePunc = strings.NewReplacer("(", "", ")", "", "*", "")

// New wraps a function for testing.
// The provided value f must be a function or method.
func New(f interface{}) *Func {
	fv := reflect.ValueOf(f)
	if fv.Kind() != reflect.Func {
		panic("argument to New must be a func")
	}
	var name string
	rf := runtime.FuncForPC(fv.Pointer())
	if rf != nil {
		name = rf.Name()
		if methType := strings.LastIndex(name, ".("); methType != -1 {
			name = removePunc.Replace(name[methType+2:])
		} else if lastDot := strings.LastIndex(name, "."); lastDot != -1 {
			name = name[lastDot+1:]
			if strings.HasPrefix(name, "func") {
				// Looks like some anonymous function. Prefer naming it "f".
				name = "f"
			}
		}
	} else {
		name = "f"
	}

	return &Func{
		f:    f,
		fv:   fv,
		Name: name,
	}
}

// Result is the result of a function call, for use with Check.
type Result struct {
	// Result is the return value(s) of the function.
	Result []interface{}

	// Panic is the panic value of the function.
	Panic interface{}

	// Panicked is whether the function paniced.
	// It can be used to determine whether a function
	// called panic(nil).
	Panicked bool
}

// Case is a test case to run.
//
// Test cases can be either named or unnamed, depending on how they're
// created. Naming cases is optional; all failures messages aim to
// have useful output and include the input to the function.
//
// Unless the function's arity is zero, all cases should have their input
// set with In.
//
// The case's expected output can be set with Want and/or Check.
type Case struct {
	f        *Func
	in       []interface{}
	name     string        // optional
	want     []interface{} // non-nil if we check args
	checkRes []func(Result) error
}

// Case returns a new named case. It should be modified before use.
func (f *Func) Case(name string) *Case {
	return &Case{f: f, name: name}
}

// In returns a new unnamed test case. It will be identified by its arguments
// only.
func (f *Func) In(args ...interface{}) *Case {
	return &Case{f: f, in: args}
}

// In sets the arguments of c used to call f.
func (c *Case) In(args ...interface{}) *Case {
	c.in = args
	return c
}

// Want sets the expected result values of the test case.
// Want modifies and returns c.
// Callers my use both Want and Check.
func (c *Case) Want(result ...interface{}) *Case {
	if c.want != nil {
		panic("duplicate Want declared on functest.Case")
	}
	c.want = result
	numOut := c.f.fv.Type().NumOut()
	if len(result) != numOut {
		// TODO: let caller providing only interesting result values, or
		// provide matchers.
		panic(fmt.Sprintf("Want called with %d values; function returns %d values", len(result), numOut))
	}
	return c
}

// Check adds a function to check the result of the case's function
// call. It is a low-level function when Want is insufficient.
// For instance, it allows checking whether a function panics.
// If no checker functions are registered, function panics are considered
// a test failure.
//
// Check modifies and returns c.
// Callers my use both Want and Check, and may use Check multiple times.
func (c *Case) Check(checker func(Result) error) *Case {
	c.checkRes = append(c.checkRes, checker)
	return c
}

// Test runs the provided test cases against f.
// If any test cases fail, t.Errorf is called.
func (f *Func) Test(t testing.TB, cases ...*Case) {
	for _, tc := range cases {
		f.testCase(t, tc)
	}
}

func (f *Func) checkCall(in []reflect.Value) (out []reflect.Value, didPanic bool, panicValue interface{}) {
	defer func() { panicValue = recover() }()
	didPanic = true
	out = f.fv.Call(in)
	didPanic = false
	return
}

var nilEmptyInterface = reflect.Zero(reflect.TypeOf((*interface{})(nil)).Elem())

func (f *Func) testCase(t testing.TB, c *Case) {
	// Non-variadic:
	ft := f.fv.Type()
	inReg := ft.NumIn()
	if ft.IsVariadic() {
		inReg--
		if len(c.in) < inReg {
			c.errorf(t, ": input has %d arguments; func requires at least %d", len(c.in), inReg)
			return
		}
	} else if len(c.in) != ft.NumIn() {
		c.errorf(t, ": input has %d arguments; func takes %d", len(c.in), ft.NumIn())
		return
	}

	inv := make([]reflect.Value, len(c.in))
	for i, v := range c.in {
		if v == nil {
			inv[i] = nilEmptyInterface
		} else {
			inv[i] = reflect.ValueOf(v)
		}
	}
	got, didPanic, panicValue := f.checkCall(inv)

	var goti []interface{}
	if !didPanic {
		goti = make([]interface{}, len(got))
		for i, rv := range got {
			goti[i] = rv.Interface()
		}
	}

	if c.want != nil {
		if !reflect.DeepEqual(goti, c.want) {
			c.errorf(t, " = %v; want %v", formatRes(goti), formatRes(c.want))
		}
	}
	for _, checkRes := range c.checkRes {
		err := checkRes(Result{
			Result:   goti,
			Panic:    panicValue,
			Panicked: didPanic,
		})
		if err != nil {
			c.errorf(t, ": %v", err)
		}
	}
	if didPanic && (c.checkRes == nil) {
		c.errorf(t, ": panicked with %v", panicValue)
	}
}

func formatRes(res []interface{}) string {
	var buf bytes.Buffer
	if len(res) != 1 {
		buf.WriteByte('(')
	}
	formatValues(&buf, res)
	if len(res) != 1 {
		buf.WriteByte(')')
	}
	return buf.String()
}

func formatValues(buf *bytes.Buffer, vals []interface{}) {
	for i, v := range vals {
		if i != 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(buf, "%#v", v)
	}
}

func (c *Case) errorf(t testing.TB, format string, args ...interface{}) {
	var buf bytes.Buffer
	if c.name != "" {
		fmt.Fprintf(&buf, "%s: ", c.name)
	}
	buf.WriteString(c.f.Name)
	buf.WriteString("(")
	formatValues(&buf, c.in)
	buf.WriteString(")")
	fmt.Fprintf(&buf, format, args...)
	t.Errorf("%s", buf.Bytes())
}
