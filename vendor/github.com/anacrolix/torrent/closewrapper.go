package torrent

import "net"

type closeWrapper struct {
	net.Conn
	closer func() error
}

func (me closeWrapper) Close() error {
	return me.closer()
}
