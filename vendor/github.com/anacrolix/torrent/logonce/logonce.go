// Package logonce implements an io.Writer facade that only performs distinct
// writes. This can be used by log.Loggers as they're guaranteed to make a
// single Write method call for each message. This is useful for loggers that
// print useful information about unexpected conditions that aren't fatal in
// code.
package logonce

import (
	"io"
	"log"
	"os"
)

// A default logger similar to the default logger in the log package.
var Stderr *log.Logger

func init() {
	// This should emulate the default logger in the log package where
	// possible. No time flag so that messages don't differ by time. Code
	// debug information is useful.
	Stderr = log.New(Writer(os.Stderr), "logonce: ", log.Lshortfile)
}

type writer struct {
	w      io.Writer
	writes map[string]struct{}
}

func (w writer) Write(p []byte) (n int, err error) {
	s := string(p)
	if _, ok := w.writes[s]; ok {
		return
	}
	n, err = w.w.Write(p)
	if n != len(s) {
		s = string(p[:n])
	}
	w.writes[s] = struct{}{}
	return
}

func Writer(w io.Writer) io.Writer {
	return writer{
		w:      w,
		writes: make(map[string]struct{}),
	}
}
