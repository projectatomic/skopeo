package mmap_span

type sizer interface {
	Size() int64
}

type span []sizer

func (s span) ApplyTo(off int64, f func(int64, sizer) (stop bool)) {
	for _, interval := range s {
		iSize := interval.Size()
		if off >= iSize {
			off -= iSize
		} else {
			if f(off, interval) {
				return
			}
			off = 0
		}
	}
}
