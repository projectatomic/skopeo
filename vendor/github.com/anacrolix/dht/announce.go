package dht

// get_peers and announce_peers.

import (
	"container/heap"
	"context"

	"github.com/anacrolix/dht/krpc"
	"github.com/anacrolix/sync"
	"github.com/willf/bloom"
	"golang.org/x/time/rate"
)

// Maintains state for an ongoing Announce operation. An Announce is started
// by calling Server.Announce.
type Announce struct {
	mu    sync.Mutex
	Peers chan PeersValues
	// Inner chan is set to nil when on close.
	values     chan PeersValues
	stop       chan struct{}
	triedAddrs *bloom.BloomFilter
	// True when contact with all starting addrs has been initiated. This
	// prevents a race where the first transaction finishes before the rest
	// have been opened, sees no other transactions are pending and ends the
	// announce.
	contactedStartAddrs bool
	// How many transactions are still ongoing.
	pending  int
	server   *Server
	infoHash int160
	// Count of (probably) distinct addresses we've sent get_peers requests
	// to.
	numContacted int
	// The torrent port that we're announcing.
	announcePort int
	// The torrent port should be determined by the receiver in case we're
	// being NATed.
	announcePortImplied bool

	nodesPendingContact nodesByDistance
	nodeContactorCond   sync.Cond
	contactRateLimiter  *rate.Limiter
}

// Returns the number of distinct remote addresses the announce has queried.
func (a *Announce) NumContacted() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.numContacted
}

func newBloomFilterForTraversal() *bloom.BloomFilter {
	return bloom.NewWithEstimates(10000, 0.5)
}

// This is kind of the main thing you want to do with DHT. It traverses the
// graph toward nodes that store peers for the infohash, streaming them to the
// caller, and announcing the local node to each node if allowed and
// specified.
func (s *Server) Announce(infoHash [20]byte, port int, impliedPort bool) (*Announce, error) {
	startAddrs, err := s.traversalStartingAddrs()
	if err != nil {
		return nil, err
	}
	a := &Announce{
		Peers:               make(chan PeersValues, 100),
		stop:                make(chan struct{}),
		values:              make(chan PeersValues),
		triedAddrs:          newBloomFilterForTraversal(),
		server:              s,
		infoHash:            int160FromByteArray(infoHash),
		announcePort:        port,
		announcePortImplied: impliedPort,
		contactRateLimiter:  s.announceContactRateLimiter,
	}
	a.nodesPendingContact.target = int160FromByteArray(infoHash)
	a.nodeContactorCond.L = &a.mu
	// Function ferries from values to Values until discovery is halted.
	go func() {
		defer close(a.Peers)
		for {
			select {
			case psv := <-a.values:
				select {
				case a.Peers <- psv:
				case <-a.stop:
					return
				}
			case <-a.stop:
				return
			}
		}
	}()
	go func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		for _, addr := range startAddrs {
			if !a.reserveContact(addr) {
				continue
			}
			a.mu.Unlock()
			a.contactRateLimiter.Wait(context.TODO())
			a.mu.Lock()
			a.contact(addr)
		}
		a.contactedStartAddrs = true
		// If we failed to contact any of the starting addrs, no transactions
		// will complete triggering a check that there are no pending
		// responses.
		a.maybeClose()
	}()
	go a.nodeContactor()
	return a, nil
}

func validNodeAddr(addr Addr) bool {
	ua := addr.UDPAddr()
	if ua.Port == 0 {
		return false
	}
	if ip4 := ua.IP.To4(); ip4 != nil && ip4[0] == 0 {
		// Why?
		return false
	}
	return true
}

func (a *Announce) reserveContact(addr Addr) bool {
	if !validNodeAddr(addr) {
		return false
	}
	if a.triedAddrs.Test([]byte(addr.String())) {
		return false
	}
	if a.server.ipBlocked(addr.UDPAddr().IP) {
		return false
	}
	a.pending++
	a.triedAddrs.Add([]byte(addr.String()))
	return true
}

func (a *Announce) completeContact() {
	a.pending--
	a.maybeClose()
}

func (a *Announce) contact(addr Addr) {
	a.numContacted++
	go func() {
		if a.getPeers(addr) != nil {
			a.mu.Lock()
			a.completeContact()
			a.mu.Unlock()
		}
	}()
}

func (a *Announce) maybeClose() {
	if a.contactedStartAddrs && a.pending == 0 {
		a.close()
	}
}

func (a *Announce) responseNode(node krpc.NodeInfo) {
	a.pendContact(node)
}

// Announce to a peer, if appropriate.
func (a *Announce) maybeAnnouncePeer(to Addr, token string, peerId *krpc.ID) {
	if !a.server.config.NoSecurity && (peerId == nil || !NodeIdSecure(*peerId, to.UDPAddr().IP)) {
		return
	}
	a.server.mu.Lock()
	defer a.server.mu.Unlock()
	a.server.announcePeer(to, a.infoHash, a.announcePort, token, a.announcePortImplied, nil)
}

func (a *Announce) getPeers(addr Addr) error {
	a.server.mu.Lock()
	defer a.server.mu.Unlock()
	return a.server.getPeers(addr, a.infoHash, func(m krpc.Msg, err error) {
		// Register suggested nodes closer to the target info-hash.
		if m.R != nil && m.SenderID() != nil {
			expvars.Add("announce get_peers response nodes values", int64(len(m.R.Nodes)))
			expvars.Add("announce get_peers response nodes6 values", int64(len(m.R.Nodes6)))
			a.mu.Lock()
			m.R.ForAllNodes(a.responseNode)
			a.mu.Unlock()
			select {
			case a.values <- PeersValues{
				Peers: m.R.Values,
				NodeInfo: krpc.NodeInfo{
					Addr: addr.KRPC(),
					ID:   *m.SenderID(),
				},
			}:
			case <-a.stop:
			}
			a.maybeAnnouncePeer(addr, m.R.Token, m.SenderID())
		}
		a.mu.Lock()
		a.completeContact()
		a.mu.Unlock()
	})
}

// Corresponds to the "values" key in a get_peers KRPC response. A list of
// peers that a node has reported as being in the swarm for a queried info
// hash.
type PeersValues struct {
	Peers         []Peer // Peers given in get_peers response.
	krpc.NodeInfo        // The node that gave the response.
}

// Stop the announce.
func (a *Announce) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.close()
}

func (a *Announce) close() {
	select {
	case <-a.stop:
	default:
		close(a.stop)
		a.nodeContactorCond.Broadcast()
	}
}

func (a *Announce) pendContact(node krpc.NodeInfo) {
	if !a.reserveContact(NewAddr(node.Addr.UDP())) {
		return
	}
	heap.Push(&a.nodesPendingContact, node)
	a.nodeContactorCond.Signal()
}

func (a *Announce) nodeContactor() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for {
		for {
			select {
			case <-a.stop:
				return
			default:
			}
			if a.nodesPendingContact.Len() > 0 {
				break
			}
			a.nodeContactorCond.Wait()
		}
		a.mu.Unlock()
		a.contactRateLimiter.Wait(context.TODO())
		a.mu.Lock()
		ni := heap.Pop(&a.nodesPendingContact).(krpc.NodeInfo)
		a.contact(NewAddr(ni.Addr.UDP()))
	}
}
