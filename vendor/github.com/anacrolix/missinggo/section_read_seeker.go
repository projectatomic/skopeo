package missinggo

import (
	"context"
	"fmt"
	"io"
)

type sectionReadSeeker struct {
	base      io.ReadSeeker
	off, size int64
}

type ReadSeekContexter interface {
	io.ReadSeeker
	ReadContexter
}

// Returns a ReadSeeker on a section of another ReadSeeker.
func NewSectionReadSeeker(base io.ReadSeeker, off, size int64) (ret ReadSeekContexter) {
	ret = &sectionReadSeeker{
		base: base,
		off:  off,
		size: size,
	}
	seekOff, err := ret.Seek(0, io.SeekStart)
	if err != nil {
		panic(err)
	}
	if seekOff != 0 {
		panic(seekOff)
	}
	return
}

func (me *sectionReadSeeker) Seek(off int64, whence int) (ret int64, err error) {
	switch whence {
	case io.SeekStart:
		off += me.off
	case io.SeekCurrent:
	case io.SeekEnd:
		off += me.off + me.size
		whence = io.SeekStart
	default:
		err = fmt.Errorf("unhandled whence: %d", whence)
		return
	}
	ret, err = me.base.Seek(off, whence)
	ret -= me.off
	return
}

func (me *sectionReadSeeker) ReadContext(ctx context.Context, b []byte) (int, error) {
	off, err := me.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	left := me.size - off
	if left <= 0 {
		return 0, io.EOF
	}
	b = LimitLen(b, left)
	if rc, ok := me.base.(ReadContexter); ok {
		return rc.ReadContext(ctx, b)
	}
	if ctx != context.Background() {
		// Can't handle cancellation.
		panic(ctx)
	}
	return me.base.Read(b)
}

func (me *sectionReadSeeker) Read(b []byte) (int, error) {
	return me.ReadContext(context.Background(), b)
}
