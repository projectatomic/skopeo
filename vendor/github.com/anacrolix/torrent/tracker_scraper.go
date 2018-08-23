package torrent

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/anacrolix/dht/krpc"
	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/torrent/tracker"
)

// Announces a torrent to a tracker at regular intervals, when peers are
// required.
type trackerScraper struct {
	u url.URL
	// Causes the trackerScraper to stop running.
	stop         missinggo.Event
	t            *Torrent
	lastAnnounce trackerAnnounceResult
}

func (ts *trackerScraper) statusLine() string {
	var w bytes.Buffer
	fmt.Fprintf(&w, "%q\t%s\t%s",
		ts.u.String(),
		func() string {
			na := time.Until(ts.lastAnnounce.Completed.Add(ts.lastAnnounce.Interval))
			if na > 0 {
				na /= time.Second
				na *= time.Second
				return na.String()
			} else {
				return "anytime"
			}
		}(),
		func() string {
			if ts.lastAnnounce.Err != nil {
				return ts.lastAnnounce.Err.Error()
			}
			if ts.lastAnnounce.Completed.IsZero() {
				return "never"
			}
			return fmt.Sprintf("%d peers", ts.lastAnnounce.NumPeers)
		}(),
	)
	return w.String()
}

type trackerAnnounceResult struct {
	Err       error
	NumPeers  int
	Interval  time.Duration
	Completed time.Time
}

func (me *trackerScraper) getIp() (ip net.IP, err error) {
	ips, err := net.LookupIP(me.u.Hostname())
	if err != nil {
		return
	}
	if len(ips) == 0 {
		err = errors.New("no ips")
		return
	}
	for _, ip = range ips {
		if me.t.cl.ipIsBlocked(ip) {
			continue
		}
		switch me.u.Scheme {
		case "udp4":
			if ip.To4() == nil {
				continue
			}
		case "udp6":
			if ip.To4() != nil {
				continue
			}
		}
		return
	}
	err = errors.New("no acceptable ips")
	return
}

func (me *trackerScraper) trackerUrl(ip net.IP) string {
	u := me.u
	if u.Port() != "" {
		u.Host = net.JoinHostPort(ip.String(), u.Port())
	}
	return u.String()
}

// Return how long to wait before trying again. For most errors, we return 5
// minutes, a relatively quick turn around for DNS changes.
func (me *trackerScraper) announce() (ret trackerAnnounceResult) {
	defer func() {
		ret.Completed = time.Now()
	}()
	ret.Interval = 5 * time.Minute
	ip, err := me.getIp()
	if err != nil {
		ret.Err = fmt.Errorf("error getting ip: %s", err)
		return
	}
	me.t.cl.lock()
	req := me.t.announceRequest()
	me.t.cl.unlock()
	res, err := tracker.Announce{
		HTTPProxy:  me.t.cl.config.HTTPProxy,
		UserAgent:  me.t.cl.config.HTTPUserAgent,
		TrackerUrl: me.trackerUrl(ip),
		Request:    req,
		HostHeader: me.u.Host,
		ServerName: me.u.Hostname(),
		UdpNetwork: me.u.Scheme,
		ClientIp4:  krpc.NodeAddr{IP: me.t.cl.config.PublicIp4},
		ClientIp6:  krpc.NodeAddr{IP: me.t.cl.config.PublicIp6},
	}.Do()
	if err != nil {
		ret.Err = fmt.Errorf("error announcing: %s", err)
		return
	}
	me.t.AddPeers(Peers(nil).AppendFromTracker(res.Peers))
	ret.NumPeers = len(res.Peers)
	ret.Interval = time.Duration(res.Interval) * time.Second
	return
}

func (me *trackerScraper) Run() {
	for {
		ar := me.announce()
		me.t.cl.lock()
		me.lastAnnounce = ar
		me.t.cl.unlock()

	wait:
		interval := ar.Interval
		if interval < time.Minute {
			interval = time.Minute
		}
		wantPeers := me.t.wantPeersEvent.LockedChan(me.t.cl.locker())
		select {
		case <-wantPeers:
			if interval > time.Minute {
				interval = time.Minute
			}
			wantPeers = nil
		default:
		}

		select {
		case <-me.t.closed.LockedChan(me.t.cl.locker()):
			return
		case <-me.stop.LockedChan(me.t.cl.locker()):
			return
		case <-wantPeers:
			goto wait
		case <-time.After(time.Until(ar.Completed.Add(interval))):
		}
	}
}
