package mmsg

import (
	"net"

	"github.com/anacrolix/missinggo/expect"

	"github.com/anacrolix/mmsg/socket"
)

// Considered MSG_DONTWAIT, but I think Go puts the socket into non-blocking
// mode in its runtime and it seems to do the right thing.
const flags = 0

type Conn struct {
	err error
	s   *socket.Conn
	pr  PacketReader
}

type PacketReader interface {
	ReadFrom([]byte) (int, net.Addr, error)
}

func NewConn(pr PacketReader) *Conn {
	ret := Conn{
		pr: pr,
	}
	ret.s, ret.err = socket.NewConn(pr)
	return &ret
}

func (me *Conn) recvMsgAsMsgs(ms []Message) (int, error) {
	err := me.RecvMsg(&ms[0])
	if err != nil {
		return 0, err
	}
	return 1, err
}

func (me *Conn) RecvMsgs(ms []Message) (n int, err error) {
	// defer func() {
	// 	log.Println(n, err)
	// }()
	if me.err != nil {
		return me.recvMsgAsMsgs(ms)
	}
	sms := make([]socket.Message, len(ms))
	for i := range ms {
		sms[i].Buffers = ms[i].Buffers
	}
	n, err = me.s.RecvMsgs(sms, flags)
	if err != nil && err.Error() == "not implemented" {
		expect.Nil(me.err)
		me.err = err
		if n <= 0 {
			return me.recvMsgAsMsgs(ms)
		}
		err = nil
	}
	for i := 0; i < n; i++ {
		ms[i].Addr = sms[i].Addr
		ms[i].N = sms[i].N
	}
	return n, err
}

func (me *Conn) RecvMsg(m *Message) error {
	if len(m.Buffers) == 1 { // What about 0?
		var err error
		m.N, m.Addr, err = me.pr.ReadFrom(m.Buffers[0])
		return err
	}
	sm := socket.Message{
		Buffers: m.Buffers,
	}
	err := me.s.RecvMsg(&sm, flags)
	m.Addr = sm.Addr
	m.N = sm.N
	return err
}

type Message struct {
	Buffers [][]byte
	N       int
	Addr    net.Addr
}

func (me *Message) Payload() (p []byte) {
	n := me.N
	for _, b := range me.Buffers {
		if len(b) >= n {
			p = append(p, b[:n]...)
			return
		}
		p = append(p, b...)
		n -= len(b)
	}
	panic(n)
}

func (me *Conn) Err() error {
	return me.err
}
