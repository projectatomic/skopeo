package tracker

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/anacrolix/dht/krpc"
)

// Marshalled as binary by the UDP client, so be careful making changes.
type AnnounceRequest struct {
	InfoHash   [20]byte
	PeerId     [20]byte
	Downloaded int64
	Left       uint64
	Uploaded   int64
	// Apparently this is optional. None can be used for announces done at
	// regular intervals.
	Event     AnnounceEvent
	IPAddress uint32
	Key       int32
	NumWant   int32 // How many peer addresses are desired. -1 for default.
	Port      uint16
} // 82 bytes

type AnnounceResponse struct {
	Interval int32 // Minimum seconds the local peer should wait before next announce.
	Leechers int32
	Seeders  int32
	Peers    []Peer
}

type AnnounceEvent int32

func (e AnnounceEvent) String() string {
	// See BEP 3, "event".
	return []string{"empty", "completed", "started", "stopped"}[e]
}

const (
	None      AnnounceEvent = iota
	Completed               // The local peer just completed the torrent.
	Started                 // The local peer has just resumed this torrent.
	Stopped                 // The local peer is leaving the swarm.
)

var (
	ErrBadScheme = errors.New("unknown scheme")
)

type Announce struct {
	TrackerUrl string
	Request    AnnounceRequest
	HostHeader string
	HTTPProxy  func(*http.Request) (*url.URL, error)
	ServerName string
	UserAgent  string
	UdpNetwork string
	// If the port is zero, it's assumed to be the same as the Request.Port.
	ClientIp4 krpc.NodeAddr
	// If the port is zero, it's assumed to be the same as the Request.Port.
	ClientIp6 krpc.NodeAddr
	Context   context.Context
}

func (me Announce) Do() (res AnnounceResponse, err error) {
	_url, err := url.Parse(me.TrackerUrl)
	if err != nil {
		return
	}
	switch _url.Scheme {
	case "http", "https":
		return announceHTTP(me, _url)
	case "udp", "udp4", "udp6":
		return announceUDP(me, _url)
	default:
		err = ErrBadScheme
		return
	}
}
