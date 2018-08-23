package tracker

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"

	"github.com/anacrolix/dht/krpc"
	"github.com/anacrolix/missinggo"
)

type torrent struct {
	Leechers int32
	Seeders  int32
	Peers    []krpc.NodeAddr
}

type server struct {
	pc    net.PacketConn
	conns map[int64]struct{}
	t     map[[20]byte]torrent
}

func marshal(parts ...interface{}) (ret []byte, err error) {
	var buf bytes.Buffer
	for _, p := range parts {
		err = binary.Write(&buf, binary.BigEndian, p)
		if err != nil {
			return
		}
	}
	ret = buf.Bytes()
	return
}

func (s *server) respond(addr net.Addr, rh ResponseHeader, parts ...interface{}) (err error) {
	b, err := marshal(append([]interface{}{rh}, parts...)...)
	if err != nil {
		return
	}
	_, err = s.pc.WriteTo(b, addr)
	return
}

func (s *server) newConn() (ret int64) {
	ret = rand.Int63()
	if s.conns == nil {
		s.conns = make(map[int64]struct{})
	}
	s.conns[ret] = struct{}{}
	return
}

func (s *server) serveOne() (err error) {
	b := make([]byte, 0x10000)
	n, addr, err := s.pc.ReadFrom(b)
	if err != nil {
		return
	}
	r := bytes.NewReader(b[:n])
	var h RequestHeader
	err = readBody(r, &h)
	if err != nil {
		return
	}
	switch h.Action {
	case ActionConnect:
		if h.ConnectionId != connectRequestConnectionId {
			return
		}
		connId := s.newConn()
		err = s.respond(addr, ResponseHeader{
			ActionConnect,
			h.TransactionId,
		}, ConnectionResponse{
			connId,
		})
		return
	case ActionAnnounce:
		if _, ok := s.conns[h.ConnectionId]; !ok {
			s.respond(addr, ResponseHeader{
				TransactionId: h.TransactionId,
				Action:        ActionError,
			}, []byte("not connected"))
			return
		}
		var ar AnnounceRequest
		err = readBody(r, &ar)
		if err != nil {
			return
		}
		t := s.t[ar.InfoHash]
		bm := func() encoding.BinaryMarshaler {
			ip := missinggo.AddrIP(addr)
			if ip.To4() != nil {
				return krpc.CompactIPv4NodeAddrs(t.Peers)
			}
			return krpc.CompactIPv6NodeAddrs(t.Peers)
		}()
		b, err = bm.MarshalBinary()
		if err != nil {
			panic(err)
		}
		err = s.respond(addr, ResponseHeader{
			TransactionId: h.TransactionId,
			Action:        ActionAnnounce,
		}, AnnounceResponseHeader{
			Interval: 900,
			Leechers: t.Leechers,
			Seeders:  t.Seeders,
		}, b)
		return
	default:
		err = fmt.Errorf("unhandled action: %d", h.Action)
		s.respond(addr, ResponseHeader{
			TransactionId: h.TransactionId,
			Action:        ActionError,
		}, []byte("unhandled action"))
		return
	}
}
