package functest

import (
	"bytes"
	"fmt"
	"testing"
)

// trec is a testing.TB which logs Errorf calls to buf
type trec struct {
	testing.TB // crash on unimplemented methods
	buf        bytes.Buffer
}

func (t *trec) Errorf(format string, args ...interface{}) {
	t.buf.WriteString("ERR: ")
	fmt.Fprintf(&t.buf, format, args...)
	t.buf.WriteByte('\n')
}

func (t *trec) Logf(format string, args ...interface{}) {
	t.buf.WriteString("LOG: ")
	fmt.Fprintf(&t.buf, format, args...)
	t.buf.WriteByte('\n')
}

func (t *trec) String() string { return t.buf.String() }

func add(a, b int) int { return a + b }

func TestBasic(t *testing.T) {
	f := New(add)
	trec := new(trec)
	f.Test(trec,
		f.In(1, 2).Want(3),
		f.In(5, 6).Want(100),
		f.Case("also wrong").In(5, 6).Want(101),
	)
	want := `ERR: add(5, 6) = 11; want 100
ERR: also wrong: add(5, 6) = 11; want 101
`
	if got := trec.String(); got != want {
		t.Errorf("Output mismatch.\nGot:\n%v\nWant:\n%v\n", got, want)
	}
}

func TestBasic_Strings(t *testing.T) {
	concat := func(a, b string) string { return a + b }
	f := New(concat)
	f.Name = "concat"
	trec := new(trec)
	f.Test(trec,
		f.In("a", "b").Want("ab"),
		f.In("a", "b\x00").Want("ab"),
	)
	want := `ERR: concat("a", "b\x00") = "ab\x00"; want "ab"
`
	if got := trec.String(); got != want {
		t.Errorf("Output mismatch.\nGot:\n%v\nWant:\n%v\n", got, want)
	}
}

func TestVariadic(t *testing.T) {
	sumVar := func(vals ...int) (sum int) {
		for _, v := range vals {
			sum += v
		}
		return
	}

	f := New(sumVar)
	f.Name = "sumVar"
	trec := new(trec)
	f.Test(trec,
		f.In().Want(0),
		f.In().Want(100),
		f.In(1).Want(1),
		f.In(1).Want(100),
		f.In(1, 2).Want(3),
		f.In(1, 2, 3).Want(6),
		f.In(1, 2, 3).Want(100),
	)
	want := `ERR: sumVar() = 0; want 100
ERR: sumVar(1) = 1; want 100
ERR: sumVar(1, 2, 3) = 6; want 100
`
	if got := trec.String(); got != want {
		t.Errorf("Output mismatch.\nGot:\n%v\nWant:\n%v\n", got, want)
	}
}

func condPanic(doPanic bool, panicValue interface{}) {
	if doPanic {
		panic(panicValue)
	}
}

func TestPanic(t *testing.T) {
	f := New(condPanic)
	f.Name = "condPanic"
	trec := new(trec)
	f.Test(trec,
		f.In(false, nil),
		f.In(true, "boom").Check(func(res Result) error {
			trec.Logf("Got res: %+v", res)
			if res.Panic != "boom" {
				return fmt.Errorf("panic = %v; want boom", res.Panic)
			}
			return nil
		}),
		f.Case("panic with nil").In(true, nil),
	)
	want := `LOG: Got res: {Result:[] Panic:boom Panicked:true}
ERR: panic with nil: condPanic(true, <nil>): panicked with <nil>
`
	if got := trec.String(); got != want {
		t.Errorf("Output mismatch.\nGot:\n%v\nWant:\n%v\n", got, want)
	}
}

func TestName_AutoFunc(t *testing.T) {
	testName(t, New(add), "add")
}

type SomeType struct{}

func (t *SomeType) SomeMethod(int) int { return 123 }

func TestName_AutoMethod(t *testing.T) {
	testName(t, New((*SomeType).SomeMethod), "SomeType.SomeMethod")
}

func testName(t *testing.T, f *Func, want string) {
	if f.Name != want {
		t.Errorf("name = %q; want %q", f.Name, want)
	}
}
