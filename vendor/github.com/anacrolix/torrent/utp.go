package torrent

import (
	"context"
	"net"
)

// Abstracts the utp Socket, so the implementation can be selected from
// different packages.
type utpSocket interface {
	net.PacketConn
	// net.Listener, but we can't have duplicate Close.
	Accept() (net.Conn, error)
	Addr() net.Addr
	// net.Dialer but there's no interface.
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
	// Dial(addr string) (net.Conn, error)
}
