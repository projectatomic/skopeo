package dht

import "errors"

// Node table, with indexes on distance from root ID to bucket, and node addr.
type table struct {
	rootID  int160
	k       int
	buckets [160]bucket
	addrs   map[string]map[int160]struct{}
}

func (tbl *table) addrNodes(addr Addr) []*node {
	a := tbl.addrs[addr.String()]
	ret := make([]*node, 0, len(a))
	for id := range a {
		ret = append(ret, tbl.getNode(addr, id))
	}
	return ret
}

func (tbl *table) dropNode(n *node) {
	as := n.addr.String()
	if _, ok := tbl.addrs[as][n.id]; !ok {
		panic("missing id for addr")
	}
	delete(tbl.addrs[as], n.id)
	if len(tbl.addrs[as]) == 0 {
		delete(tbl.addrs, as)
	}
	b := tbl.bucketForID(n.id)
	if _, ok := b.nodes[n]; !ok {
		panic("expected node in bucket")
	}
	delete(b.nodes, n)
}

func (tbl *table) bucketForID(id int160) *bucket {
	return &tbl.buckets[tbl.bucketIndex(id)]
}

func (tbl *table) numNodes() (num int) {
	for _, b := range tbl.buckets {
		num += b.Len()
	}
	return
}

func (tbl *table) bucketIndex(id int160) int {
	if id == tbl.rootID {
		panic("nobody puts the root ID in a bucket")
	}
	var a int160
	a.Xor(&tbl.rootID, &id)
	index := 160 - a.BitLen()
	return index
}

func (tbl *table) forNodes(f func(*node) bool) bool {
	for _, b := range tbl.buckets {
		if !b.EachNode(f) {
			return false
		}
	}
	return true
}

func (tbl *table) getNode(addr Addr, id int160) *node {
	if id == tbl.rootID {
		return nil
	}
	return tbl.buckets[tbl.bucketIndex(id)].GetNode(addr, id)
}

func (tbl *table) closestNodes(k int, target int160, filter func(*node) bool) (ret []*node) {
	for bi := func() int {
		if target == tbl.rootID {
			return len(tbl.buckets) - 1
		} else {
			return tbl.bucketIndex(target)
		}
	}(); bi >= 0 && len(ret) < k; bi-- {
		for n := range tbl.buckets[bi].nodes {
			if filter(n) {
				ret = append(ret, n)
			}
		}
	}
	// TODO: Keep only the closest.
	if len(ret) > k {
		ret = ret[:k]
	}
	return
}

func (tbl *table) addNode(n *node) error {
	if n.id == tbl.rootID {
		return errors.New("is root id")
	}
	b := &tbl.buckets[tbl.bucketIndex(n.id)]
	if b.GetNode(n.addr, n.id) != nil {
		return errors.New("already present")
	}
	if b.Len() >= tbl.k {
		return errors.New("bucket is full")
	}
	b.AddNode(n, tbl.k)
	if tbl.addrs == nil {
		tbl.addrs = make(map[string]map[int160]struct{}, 160*tbl.k)
	}
	as := n.addr.String()
	if tbl.addrs[as] == nil {
		tbl.addrs[as] = make(map[int160]struct{}, 1)
	}
	tbl.addrs[as][n.id] = struct{}{}
	return nil
}
