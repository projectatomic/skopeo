package torrent

import (
	"net"

	"github.com/anacrolix/dht/krpc"
	"github.com/anacrolix/torrent/peer_protocol"
)

type Peer struct {
	Id     [20]byte
	IP     net.IP
	Port   int
	Source peerSource
	// Peer is known to support encryption.
	SupportsEncryption bool
	peer_protocol.PexPeerFlags
}

func (me *Peer) FromPex(na krpc.NodeAddr, fs peer_protocol.PexPeerFlags) {
	me.IP = append([]byte(nil), na.IP...)
	me.Port = na.Port
	me.Source = peerSourcePEX
	// If they prefer encryption, they must support it.
	if fs.Get(peer_protocol.PexPrefersEncryption) {
		me.SupportsEncryption = true
	}
	me.PexPeerFlags = fs
}

func (me Peer) addr() IpPort {
	return IpPort{me.IP, uint16(me.Port)}
}
