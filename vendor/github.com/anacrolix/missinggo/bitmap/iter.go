package bitmap

import "github.com/RoaringBitmap/roaring"

type Iter struct {
	ii roaring.IntIterable
}

func (me *Iter) Next() bool {
	if me == nil {
		return false
	}
	return me.ii.HasNext()
}

func (me *Iter) Value() interface{} {
	return me.ValueInt()
}

func (me *Iter) ValueInt() int {
	return int(me.ii.Next())
}

func (me *Iter) Stop() {}
