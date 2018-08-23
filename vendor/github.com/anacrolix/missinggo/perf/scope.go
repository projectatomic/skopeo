package perf

import (
	"runtime"
)

func ScopeTimer(opts ...timerOpt) func() {
	t := NewTimer(CallerName(1))
	return func() { t.Mark("returned") }
}

func ScopeTimerOk(ok *bool) func() {
	t := NewTimer(CallerName(1))
	return func() { t.MarkOk(*ok) }
}

func ScopeTimerErr(err *error) func() {
	t := NewTimer(CallerName(1))
	return func() {
		r := recover()
		if r != nil {
			t.Mark("panic")
			panic(r)
		}
		t.MarkErr(*err)
	}
}

func CallerName(skip int) timerOpt {
	return Name(getCallerName(skip))
}

func getCallerName(skip int) string {
	var pc [1]uintptr
	runtime.Callers(3+skip, pc[:])
	fs := runtime.CallersFrames(pc[:])
	f, _ := fs.Next()
	return f.Func.Name()
}
