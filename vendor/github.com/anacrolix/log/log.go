package log

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"sort"
	"time"
)

var Default = new(Logger)

func init() {
	Default.SetHandler(&StreamHandler{
		W:   os.Stderr,
		Fmt: LineFormatter,
	})
}

type Logger struct {
	hs      map[Handler]struct{}
	values  map[interface{}]struct{}
	filters map[*Filter]struct{}
}

func (l *Logger) SetHandler(h Handler) {
	l.hs = map[Handler]struct{}{h: struct{}{}}
}

func (l *Logger) Clone() *Logger {
	ret := &Logger{
		hs:      make(map[Handler]struct{}),
		values:  make(map[interface{}]struct{}),
		filters: make(map[*Filter]struct{}),
	}
	for h, v := range l.hs {
		ret.hs[h] = v
	}
	for v, v_ := range l.values {
		ret.values[v] = v_
	}
	for f := range l.filters {
		ret.filters[f] = struct{}{}
	}
	return ret
}

func (l *Logger) AddValue(v interface{}) *Logger {
	l.values[v] = struct{}{}
	return l
}

// rename Log to allow other implementers
func (l *Logger) Handle(m Msg) {
	for v := range l.values {
		m.AddValue(v)
	}
	for f := range l.filters {
		if !f.ff(&m) {
			return
		}
	}
	for h := range l.hs {
		h.Emit(m)
	}
}

func (l *Logger) AddFilter(f *Filter) *Logger {
	if l.filters == nil {
		l.filters = make(map[*Filter]struct{})
	}
	l.filters[f] = struct{}{}
	return l
}

type Handler interface {
	Emit(Msg)
}

type ByteFormatter func(Msg) []byte

type StreamHandler struct {
	W   io.Writer
	Fmt ByteFormatter
}

func groupExtras(values map[interface{}]struct{}, fields map[string][]interface{}) (ret map[interface{}][]interface{}) {
	ret = make(map[interface{}][]interface{})
	for v := range values {
		ret[reflect.TypeOf(v)] = append(ret[reflect.TypeOf(v)], v)
	}
	for f, vs := range fields {
		ret[f] = append(ret[f], vs...)
	}
	return
}

type extra struct {
	Key    interface{}
	Values []interface{}
}

func sortExtras(extras map[interface{}][]interface{}) (ret []extra) {
	for k, v := range extras {
		ret = append(ret, extra{k, v})
	}
	sort.Slice(ret, func(i, j int) bool {
		return fmt.Sprint(ret[i].Key) < fmt.Sprint(ret[j].Key)
	})
	return
}

func LineFormatter(msg Msg) []byte {
	ret := []byte(fmt.Sprintf(
		"%s %s: %s%s",
		time.Now().Format("2006-01-02 15:04:05"),
		humanPc(msg.callers[0]),
		msg.text,
		func() string {
			extras := groupExtras(msg.values, msg.fields)
			if len(extras) == 0 {
				return ""
			} else {
				return fmt.Sprintf(", %v", sortExtras(extras))
			}
		}(),
	))
	if ret[len(ret)-1] != '\n' {
		ret = append(ret, '\n')
	}
	return ret
}

func (me *StreamHandler) Emit(msg Msg) {
	me.W.Write(me.Fmt(msg))
}

func Printf(format string, a ...interface{}) {
	Default.Handle(Fmsg(format, a...).Skip(1))
}

func Print(v ...interface{}) {
	Default.Handle(Str(fmt.Sprint(v...)).Skip(1))
}

func Call() Msg {
	var pc [1]uintptr
	n := runtime.Callers(4, pc[:])
	fs := runtime.CallersFrames(pc[:n])
	f, _ := fs.Next()
	return Fmsg("called %q", f.Function)
}
