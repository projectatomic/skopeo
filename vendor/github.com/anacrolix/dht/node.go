package dht

import (
	"time"

	"github.com/anacrolix/dht/krpc"
)

type nodeKey struct {
	addr Addr
	id   int160
}

type node struct {
	nodeKey
	announceToken string
	readOnly      bool

	lastGotQuery    time.Time
	lastGotResponse time.Time

	consecutiveFailures int
}

func (n *node) hasAddrAndID(addr Addr, id int160) bool {
	return id == n.id && n.addr.String() == addr.String()
}

func (n *node) IsSecure() bool {
	return NodeIdSecure(n.id.AsByteArray(), n.addr.UDPAddr().IP)
}

func (n *node) idString() string {
	return n.id.ByteString()
}

func (n *node) NodeInfo() (ret krpc.NodeInfo) {
	ret.Addr = n.addr.KRPC()
	if n := copy(ret.ID[:], n.idString()); n != 20 {
		panic(n)
	}
	return
}

// Per the spec in BEP 5.
func (n *node) IsGood() bool {
	if n.id.IsZero() {
		return false
	}
	if time.Since(n.lastGotResponse) < 15*time.Minute {
		return true
	}
	if !n.lastGotResponse.IsZero() && time.Since(n.lastGotQuery) < 15*time.Minute {
		return true
	}
	return false
}
