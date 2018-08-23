package tracker

import (
	"bytes"
	"context"
	"encoding"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"time"

	"github.com/anacrolix/dht/krpc"
	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/missinggo/pproffd"
	"github.com/pkg/errors"
)

type Action int32

const (
	ActionConnect Action = iota
	ActionAnnounce
	ActionScrape
	ActionError

	connectRequestConnectionId = 0x41727101980

	// BEP 41
	optionTypeEndOfOptions = 0
	optionTypeNOP          = 1
	optionTypeURLData      = 2
)

type ConnectionRequest struct {
	ConnectionId int64
	Action       int32
	TransctionId int32
}

type ConnectionResponse struct {
	ConnectionId int64
}

type ResponseHeader struct {
	Action        Action
	TransactionId int32
}

type RequestHeader struct {
	ConnectionId  int64
	Action        Action
	TransactionId int32
} // 16 bytes

type AnnounceResponseHeader struct {
	Interval int32
	Leechers int32
	Seeders  int32
}

func newTransactionId() int32 {
	return int32(rand.Uint32())
}

func timeout(contiguousTimeouts int) (d time.Duration) {
	if contiguousTimeouts > 8 {
		contiguousTimeouts = 8
	}
	d = 15 * time.Second
	for ; contiguousTimeouts > 0; contiguousTimeouts-- {
		d *= 2
	}
	return
}

type udpAnnounce struct {
	contiguousTimeouts   int
	connectionIdReceived time.Time
	connectionId         int64
	socket               net.Conn
	url                  url.URL
	a                    *Announce
}

func (c *udpAnnounce) Close() error {
	if c.socket != nil {
		return c.socket.Close()
	}
	return nil
}

func (c *udpAnnounce) ipv6() bool {
	if c.a.UdpNetwork == "udp6" {
		return true
	}
	rip := missinggo.AddrIP(c.socket.RemoteAddr())
	return rip.To16() != nil && rip.To4() == nil
}

func (c *udpAnnounce) Do(req AnnounceRequest) (res AnnounceResponse, err error) {
	err = c.connect()
	if err != nil {
		return
	}
	reqURI := c.url.RequestURI()
	if c.ipv6() {
		// BEP 15
		req.IPAddress = 0
	} else if req.IPAddress == 0 && c.a.ClientIp4.IP != nil {
		req.IPAddress = binary.BigEndian.Uint32(c.a.ClientIp4.IP.To4())
	}
	// Clearly this limits the request URI to 255 bytes. BEP 41 supports
	// longer but I'm not fussed.
	options := append([]byte{optionTypeURLData, byte(len(reqURI))}, []byte(reqURI)...)
	b, err := c.request(ActionAnnounce, req, options)
	if err != nil {
		return
	}
	var h AnnounceResponseHeader
	err = readBody(b, &h)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		err = fmt.Errorf("error parsing announce response: %s", err)
		return
	}
	res.Interval = h.Interval
	res.Leechers = h.Leechers
	res.Seeders = h.Seeders
	nas := func() interface {
		encoding.BinaryUnmarshaler
		NodeAddrs() []krpc.NodeAddr
	} {
		if c.ipv6() {
			return &krpc.CompactIPv6NodeAddrs{}
		} else {
			return &krpc.CompactIPv4NodeAddrs{}
		}
	}()
	err = nas.UnmarshalBinary(b.Bytes())
	if err != nil {
		return
	}
	for _, cp := range nas.NodeAddrs() {
		res.Peers = append(res.Peers, Peer{}.FromNodeAddr(cp))
	}
	return
}

// body is the binary serializable request body. trailer is optional data
// following it, such as for BEP 41.
func (c *udpAnnounce) write(h *RequestHeader, body interface{}, trailer []byte) (err error) {
	var buf bytes.Buffer
	err = binary.Write(&buf, binary.BigEndian, h)
	if err != nil {
		panic(err)
	}
	if body != nil {
		err = binary.Write(&buf, binary.BigEndian, body)
		if err != nil {
			panic(err)
		}
	}
	_, err = buf.Write(trailer)
	if err != nil {
		return
	}
	n, err := c.socket.Write(buf.Bytes())
	if err != nil {
		return
	}
	if n != buf.Len() {
		panic("write should send all or error")
	}
	return
}

func read(r io.Reader, data interface{}) error {
	return binary.Read(r, binary.BigEndian, data)
}

func write(w io.Writer, data interface{}) error {
	return binary.Write(w, binary.BigEndian, data)
}

// args is the binary serializable request body. trailer is optional data
// following it, such as for BEP 41.
func (c *udpAnnounce) request(action Action, args interface{}, options []byte) (*bytes.Buffer, error) {
	tid := newTransactionId()
	if err := errors.Wrap(
		c.write(
			&RequestHeader{
				ConnectionId:  c.connectionId,
				Action:        action,
				TransactionId: tid,
			}, args, options),
		"writing request",
	); err != nil {
		return nil, err
	}
	c.socket.SetReadDeadline(time.Now().Add(timeout(c.contiguousTimeouts)))
	b := make([]byte, 0x800) // 2KiB
	for {
		var (
			n        int
			readErr  error
			readDone = make(chan struct{})
		)
		go func() {
			defer close(readDone)
			n, readErr = c.socket.Read(b)
		}()
		ctx := c.a.Context
		if ctx == nil {
			ctx = context.Background()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-readDone:
		}
		if opE, ok := readErr.(*net.OpError); ok && opE.Timeout() {
			c.contiguousTimeouts++
		}
		if readErr != nil {
			return nil, errors.Wrap(readErr, "reading from socket")
		}
		buf := bytes.NewBuffer(b[:n])
		var h ResponseHeader
		err := binary.Read(buf, binary.BigEndian, &h)
		switch err {
		default:
			panic(err)
		case io.ErrUnexpectedEOF, io.EOF:
			continue
		case nil:
		}
		if h.TransactionId != tid {
			continue
		}
		c.contiguousTimeouts = 0
		if h.Action == ActionError {
			err = errors.New(buf.String())
		}
		return buf, err
	}
}

func readBody(r io.Reader, data ...interface{}) (err error) {
	for _, datum := range data {
		err = binary.Read(r, binary.BigEndian, datum)
		if err != nil {
			break
		}
	}
	return
}

func (c *udpAnnounce) connected() bool {
	return !c.connectionIdReceived.IsZero() && time.Now().Before(c.connectionIdReceived.Add(time.Minute))
}

func (c *udpAnnounce) dialNetwork() string {
	if c.a.UdpNetwork != "" {
		return c.a.UdpNetwork
	}
	return "udp"
}

func (c *udpAnnounce) connect() (err error) {
	if c.connected() {
		return nil
	}
	c.connectionId = connectRequestConnectionId
	if c.socket == nil {
		hmp := missinggo.SplitHostMaybePort(c.url.Host)
		if hmp.NoPort {
			hmp.NoPort = false
			hmp.Port = 80
		}
		c.socket, err = net.Dial(c.dialNetwork(), hmp.String())
		if err != nil {
			return
		}
		c.socket = pproffd.WrapNetConn(c.socket)
	}
	b, err := c.request(ActionConnect, nil, nil)
	if err != nil {
		return
	}
	var res ConnectionResponse
	err = readBody(b, &res)
	if err != nil {
		return
	}
	c.connectionId = res.ConnectionId
	c.connectionIdReceived = time.Now()
	return
}

// TODO: Split on IPv6, as BEP 15 says response peer decoding depends on
// network in use.
func announceUDP(opt Announce, _url *url.URL) (AnnounceResponse, error) {
	ua := udpAnnounce{
		url: *_url,
		a:   &opt,
	}
	defer ua.Close()
	return ua.Do(opt.Request)
}
