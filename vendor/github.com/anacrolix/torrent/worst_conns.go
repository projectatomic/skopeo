package torrent

import (
	"container/heap"
	"fmt"
	"unsafe"
)

func worseConn(l, r *connection) bool {
	var ml multiLess
	ml.NextBool(!l.useful(), !r.useful())
	ml.StrictNext(
		l.lastHelpful().Equal(r.lastHelpful()),
		l.lastHelpful().Before(r.lastHelpful()))
	ml.StrictNext(
		l.completedHandshake.Equal(r.completedHandshake),
		l.completedHandshake.Before(r.completedHandshake))
	ml.Next(func() (bool, bool) {
		return l.peerPriority() == r.peerPriority(), l.peerPriority() < r.peerPriority()
	})
	ml.StrictNext(l == r, uintptr(unsafe.Pointer(l)) < uintptr(unsafe.Pointer(r)))
	less, ok := ml.FinalOk()
	if !ok {
		panic(fmt.Sprintf("cannot differentiate %#v and %#v", l, r))
	}
	return less
}

type worseConnSlice struct {
	conns []*connection
}

var _ heap.Interface = &worseConnSlice{}

func (me worseConnSlice) Len() int {
	return len(me.conns)
}

func (me worseConnSlice) Less(i, j int) bool {
	return worseConn(me.conns[i], me.conns[j])
}

func (me *worseConnSlice) Pop() interface{} {
	i := len(me.conns) - 1
	ret := me.conns[i]
	me.conns = me.conns[:i]
	return ret
}

func (me *worseConnSlice) Push(x interface{}) {
	me.conns = append(me.conns, x.(*connection))
}

func (me worseConnSlice) Swap(i, j int) {
	me.conns[i], me.conns[j] = me.conns[j], me.conns[i]
}
