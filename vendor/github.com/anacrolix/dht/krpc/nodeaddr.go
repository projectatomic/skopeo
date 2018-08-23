package krpc

import (
	"bytes"
	"encoding/binary"
	"net"
	"strconv"

	"github.com/anacrolix/torrent/bencode"
)

type NodeAddr struct {
	IP   net.IP
	Port int
}

// A zero Port is taken to mean no port provided, per BEP 7.
func (me NodeAddr) String() string {
	if me.Port == 0 {
		return me.IP.String()
	}
	return net.JoinHostPort(me.IP.String(), strconv.FormatInt(int64(me.Port), 10))
}

func (me *NodeAddr) UnmarshalBinary(b []byte) error {
	me.IP = make(net.IP, len(b)-2)
	copy(me.IP, b[:len(b)-2])
	me.Port = int(binary.BigEndian.Uint16(b[len(b)-2:]))
	return nil
}

func (me *NodeAddr) UnmarshalBencode(b []byte) (err error) {
	var _b []byte
	err = bencode.Unmarshal(b, &_b)
	if err != nil {
		return
	}
	return me.UnmarshalBinary(_b)
}

func (me NodeAddr) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	b.Write(me.IP)
	binary.Write(&b, binary.BigEndian, uint16(me.Port))
	return b.Bytes(), nil
}

func (me NodeAddr) MarshalBencode() ([]byte, error) {
	return bencodeBytesResult(me.MarshalBinary())
}

func (me NodeAddr) UDP() *net.UDPAddr {
	return &net.UDPAddr{
		IP:   me.IP,
		Port: me.Port,
	}
}

func (me *NodeAddr) FromUDPAddr(ua *net.UDPAddr) {
	me.IP = ua.IP
	me.Port = ua.Port
}
