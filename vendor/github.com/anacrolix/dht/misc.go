package dht

import (
	"net"

	"github.com/anacrolix/dht/krpc"
)

func mustListen(addr string) net.PacketConn {
	ret, err := net.ListenPacket("udp", addr)
	if err != nil {
		panic(err)
	}
	return ret
}

func addrResolver(addr string) func() ([]Addr, error) {
	return func() ([]Addr, error) {
		ua, err := net.ResolveUDPAddr("udp", addr)
		return []Addr{NewAddr(ua)}, err
	}
}

type nodesByDistance struct {
	nis    []krpc.NodeInfo
	target int160
}

func (me nodesByDistance) Len() int { return len(me.nis) }
func (me nodesByDistance) Less(i, j int) bool {
	return distance(int160FromByteArray(me.nis[i].ID), me.target).Cmp(distance(int160FromByteArray(me.nis[j].ID), me.target)) < 0
}
func (me *nodesByDistance) Pop() interface{} {
	ret := me.nis[len(me.nis)-1]
	me.nis = me.nis[:len(me.nis)-1]
	return ret
}
func (me *nodesByDistance) Push(x interface{}) {
	me.nis = append(me.nis, x.(krpc.NodeInfo))
}
func (me nodesByDistance) Swap(i, j int) {
	me.nis[i], me.nis[j] = me.nis[j], me.nis[i]
}
