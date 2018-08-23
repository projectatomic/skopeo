package tracker

import (
	"net"

	"github.com/anacrolix/dht/krpc"
)

type Peer struct {
	IP   net.IP
	Port int
	ID   []byte
}

// Set from the non-compact form in BEP 3.
func (p *Peer) fromDictInterface(d map[string]interface{}) {
	p.IP = net.ParseIP(d["ip"].(string))
	p.ID = []byte(d["peer id"].(string))
	p.Port = int(d["port"].(int64))
}

func (p Peer) FromNodeAddr(na krpc.NodeAddr) Peer {
	p.IP = na.IP
	p.Port = na.Port
	return p
}
