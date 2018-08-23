package missinggo

// todo move to httptoo as ResponseRecorder

import (
	"bufio"
	"net"
	"net/http"
	"time"
)

// A http.ResponseWriter that tracks the status of the response. The status
// code, and number of bytes written for example.
type StatusResponseWriter struct {
	http.ResponseWriter
	Code         int
	BytesWritten int64
	Started      time.Time
	Ttfb         time.Duration // Time to first byte
	GotFirstByte bool
	WroteHeader  Event
	Hijacked     bool
}

var _ interface {
	http.ResponseWriter
	http.Hijacker
} = (*StatusResponseWriter)(nil)

func (me *StatusResponseWriter) Write(b []byte) (n int, err error) {
	// Exactly how it's done in the standard library. This ensures Code is
	// correct.
	if !me.WroteHeader.IsSet() {
		me.WriteHeader(http.StatusOK)
	}
	if !me.GotFirstByte && len(b) > 0 {
		if me.Started.IsZero() {
			panic("Started was not initialized")
		}
		me.Ttfb = time.Since(me.Started)
		me.GotFirstByte = true
	}
	n, err = me.ResponseWriter.Write(b)
	me.BytesWritten += int64(n)
	return
}

func (me *StatusResponseWriter) WriteHeader(code int) {
	me.ResponseWriter.WriteHeader(code)
	if !me.WroteHeader.IsSet() {
		me.Code = code
		me.WroteHeader.Set()
	}
}

func (me *StatusResponseWriter) Hijack() (c net.Conn, b *bufio.ReadWriter, err error) {
	me.Hijacked = true
	c, b, err = me.ResponseWriter.(http.Hijacker).Hijack()
	if b.Writer.Buffered() != 0 {
		panic("unexpected buffered writes")
	}
	c = responseConn{c, me}
	return
}

type responseConn struct {
	net.Conn
	s *StatusResponseWriter
}

func (me responseConn) Write(b []byte) (n int, err error) {
	n, err = me.Conn.Write(b)
	me.s.BytesWritten += int64(n)
	return
}
