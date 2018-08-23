package torrent

import (
	"context"
	"fmt"
	"io"
	"time"

	"golang.org/x/time/rate"
)

type rateLimitedReader struct {
	l *rate.Limiter
	r io.Reader

	// This is the time of the last Read's reservation.
	lastRead time.Time
}

func (me *rateLimitedReader) Read(b []byte) (n int, err error) {
	const oldStyle = false // Retained for future reference.
	if oldStyle {
		// Wait until we can read at all.
		if err := me.l.WaitN(context.Background(), 1); err != nil {
			panic(err)
		}
		// Limit the read to within the burst.
		if me.l.Limit() != rate.Inf && len(b) > me.l.Burst() {
			b = b[:me.l.Burst()]
		}
		n, err = me.r.Read(b)
		// Pay the piper.
		now := time.Now()
		me.lastRead = now
		if !me.l.ReserveN(now, n-1).OK() {
			panic(fmt.Sprintf("burst exceeded?: %d", n-1))
		}
	} else {
		// Limit the read to within the burst.
		if me.l.Limit() != rate.Inf && len(b) > me.l.Burst() {
			b = b[:me.l.Burst()]
		}
		n, err = me.r.Read(b)
		now := time.Now()
		r := me.l.ReserveN(now, n)
		if !r.OK() {
			panic(n)
		}
		me.lastRead = now
		time.Sleep(r.Delay())
	}
	return
}
