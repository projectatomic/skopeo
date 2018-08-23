package dht

import (
	"crypto"
	crand "crypto/rand"
	_ "crypto/sha1"
	"errors"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/anacrolix/dht/krpc"
	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/missinggo/conntrack"
	"github.com/anacrolix/torrent/iplist"
	"github.com/anacrolix/torrent/metainfo"
)

func defaultQueryResendDelay() time.Duration {
	return jitterDuration(5*time.Second, time.Second)
}

// Uniquely identifies a transaction to us.
type transactionKey struct {
	RemoteAddr string // host:port
	T          string // The KRPC transaction ID.
}

type StartingNodesGetter func() ([]Addr, error)

// ServerConfig allows to set up a  configuration of the `Server` instance
// to be created with NewServer
type ServerConfig struct {
	// Set NodeId Manually. Caller must ensure that if NodeId does not conform
	// to DHT Security Extensions, that NoSecurity is also set.
	NodeId [20]byte
	Conn   net.PacketConn
	// Don't respond to queries from other nodes.
	Passive       bool
	StartingNodes StartingNodesGetter
	// Disable the DHT security extension:
	// http://www.libtorrent.org/dht_sec.html.
	NoSecurity bool
	// Initial IP blocklist to use. Applied before serving and bootstrapping
	// begins.
	IPBlocklist iplist.Ranger
	// Used to secure the server's ID. Defaults to the Conn's LocalAddr(). Set
	// to the IP that remote nodes will see, as that IP is what they'll use to
	// validate our ID.
	PublicIP net.IP

	// Hook received queries. Return false if you don't want to propagate to
	// the default handlers.
	OnQuery func(query *krpc.Msg, source net.Addr) (propagate bool)
	// Called when a peer successfully announces to us.
	OnAnnouncePeer func(infoHash metainfo.Hash, peer Peer)
	// How long to wait before resending queries that haven't received a
	// response. Defaults to a random value between 4.5 and 5.5s.
	QueryResendDelay func() time.Duration
	// TODO: Expose Peers, to return NodeInfo for received get_peers queries.

	ConnectionTracking *conntrack.Instance
}

// ServerStats instance is returned by Server.Stats() and stores Server metrics
type ServerStats struct {
	// Count of nodes in the node table that responded to our last query or
	// haven't yet been queried.
	GoodNodes int
	// Count of nodes in the node table.
	Nodes int
	// Transactions awaiting a response.
	OutstandingTransactions int
	// Individual announce_peer requests that got a success response.
	SuccessfulOutboundAnnouncePeerQueries int64
	// Nodes that have been blocked.
	BadNodes                 uint
	OutboundQueriesAttempted int64
}

func jitterDuration(average time.Duration, plusMinus time.Duration) time.Duration {
	return average - plusMinus/2 + time.Duration(rand.Int63n(int64(plusMinus)))
}

type Peer = krpc.NodeAddr

func GlobalBootstrapAddrs() (addrs []Addr, err error) {
	for _, s := range []string{
		"router.utorrent.com:6881",
		"router.bittorrent.com:6881",
		"dht.transmissionbt.com:6881",
		"dht.aelitis.com:6881",     // Vuze
		"router.silotis.us:6881",   // IPv6
		"dht.libtorrent.org:25401", // @arvidn's

	} {
		host, port, err := net.SplitHostPort(s)
		if err != nil {
			panic(err)
		}
		hostAddrs, err := net.LookupHost(host)
		if err != nil {
			log.Printf("error looking up %q: %v", s, err)
			continue
		}
		for _, a := range hostAddrs {
			ua, err := net.ResolveUDPAddr("udp", net.JoinHostPort(a, port))
			if err != nil {
				log.Printf("error resolving %q: %v", a, err)
				continue
			}
			addrs = append(addrs, NewAddr(ua))
		}
	}
	if len(addrs) == 0 {
		err = errors.New("nothing resolved")
	}
	return
}

func RandomNodeID() (id [20]byte) {
	crand.Read(id[:])
	return
}

func MakeDeterministicNodeID(public net.Addr) (id [20]byte) {
	h := crypto.SHA1.New()
	h.Write([]byte(public.String()))
	h.Sum(id[:0:20])
	SecureNodeId(&id, missinggo.AddrIP(public))
	return
}
