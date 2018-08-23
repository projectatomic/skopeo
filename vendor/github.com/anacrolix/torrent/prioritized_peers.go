package torrent

import "github.com/google/btree"

// Peers are stored with their priority at insertion. Their priority may
// change if our apparent IP changes, we don't currently handle that.
type prioritizedPeersItem struct {
	prio peerPriority
	p    Peer
}

func (me prioritizedPeersItem) Less(than btree.Item) bool {
	return me.prio < than.(prioritizedPeersItem).prio
}

type prioritizedPeers struct {
	om      *btree.BTree
	getPrio func(Peer) peerPriority
}

func (me *prioritizedPeers) Each(f func(Peer)) {
	me.om.Ascend(func(i btree.Item) bool {
		f(i.(prioritizedPeersItem).p)
		return true
	})
}

func (me *prioritizedPeers) Len() int {
	return me.om.Len()
}

// Returns true if a peer is replaced.
func (me *prioritizedPeers) Add(p Peer) bool {
	return me.om.ReplaceOrInsert(prioritizedPeersItem{me.getPrio(p), p}) != nil
}

func (me *prioritizedPeers) DeleteMin() (ret prioritizedPeersItem, ok bool) {
	i := me.om.DeleteMin()
	if i == nil {
		return
	}
	ret = i.(prioritizedPeersItem)
	ok = true
	return
}

func (me *prioritizedPeers) PopMax() Peer {
	return me.om.DeleteMax().(prioritizedPeersItem).p
}
