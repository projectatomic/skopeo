package utp

import "net"

type packetConnNopCloser struct {
	net.PacketConn
}

func (packetConnNopCloser) Close() error {
	return nil
}
