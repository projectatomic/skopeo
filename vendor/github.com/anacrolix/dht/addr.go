package dht

import (
	"net"

	"github.com/anacrolix/dht/krpc"
)

// Used internally to refer to node network addresses. String() is called a
// lot, and so can be optimized. Network() is not exposed, so that the
// interface does not satisfy net.Addr, as the underlying type must be passed
// to any OS-level function that take net.Addr.
type Addr interface {
	UDPAddr() *net.UDPAddr
	String() string
	KRPC() krpc.NodeAddr
}

// Speeds up some of the commonly called Addr methods.
type cachedAddr struct {
	ua net.UDPAddr
	s  string
}

func (ca cachedAddr) String() string {
	return ca.s
}

func (ca cachedAddr) UDPAddr() *net.UDPAddr {
	return &ca.ua
}

func (ca cachedAddr) KRPC() krpc.NodeAddr {
	return krpc.NodeAddr{
		IP:   ca.ua.IP,
		Port: ca.ua.Port,
	}
}

func NewAddr(ua *net.UDPAddr) Addr {
	return cachedAddr{
		ua: *ua,
		s:  ua.String(),
	}
}
