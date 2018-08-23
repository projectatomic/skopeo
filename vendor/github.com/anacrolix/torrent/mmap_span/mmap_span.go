package mmap_span

import (
	"io"
	"log"
	"sync"

	"github.com/edsrzf/mmap-go"
)

type segment struct {
	*mmap.MMap
}

func (s segment) Size() int64 {
	return int64(len(*s.MMap))
}

type MMapSpan struct {
	mu sync.RWMutex
	span
}

func (ms *MMapSpan) Append(mmap mmap.MMap) {
	ms.span = append(ms.span, segment{&mmap})
}

func (ms *MMapSpan) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, mMap := range ms.span {
		err := mMap.(segment).Unmap()
		if err != nil {
			log.Print(err)
		}
	}
	return nil
}

func (ms *MMapSpan) Size() (ret int64) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	for _, seg := range ms.span {
		ret += seg.Size()
	}
	return
}

func (ms *MMapSpan) ReadAt(p []byte, off int64) (n int, err error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	ms.ApplyTo(off, func(intervalOffset int64, interval sizer) (stop bool) {
		_n := copy(p, (*interval.(segment).MMap)[intervalOffset:])
		p = p[_n:]
		n += _n
		return len(p) == 0
	})
	if len(p) != 0 {
		err = io.EOF
	}
	return
}

func (ms *MMapSpan) WriteAt(p []byte, off int64) (n int, err error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	ms.ApplyTo(off, func(iOff int64, i sizer) (stop bool) {
		mMap := i.(segment)
		_n := copy((*mMap.MMap)[iOff:], p)
		// err = mMap.Sync(gommap.MS_ASYNC)
		// if err != nil {
		// 	return true
		// }
		p = p[_n:]
		n += _n
		return len(p) == 0
	})
	if err != nil && len(p) != 0 {
		err = io.ErrShortWrite
	}
	return
}
