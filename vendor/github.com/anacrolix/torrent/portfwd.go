package torrent

import (
	"log"
	"time"

	flog "github.com/anacrolix/log"
	"github.com/elgatito/upnp"
)

func addPortMapping(d upnp.Device, proto upnp.Protocol, internalPort int, debug bool) {
	externalPort, err := d.AddPortMapping(proto, internalPort, internalPort, "anacrolix/torrent", 0)
	if err != nil {
		log.Printf("error adding %s port mapping: %s", proto, err)
	} else if externalPort != internalPort {
		log.Printf("external port %d does not match internal port %d in port mapping", externalPort, internalPort)
	} else if debug {
		log.Printf("forwarded external %s port %d", proto, externalPort)
	}
}

func (cl *Client) forwardPort() {
	cl.lock()
	defer cl.unlock()
	if cl.config.NoDefaultPortForwarding {
		return
	}
	cl.unlock()
	ds := upnp.Discover(0, 2*time.Second)
	cl.lock()
	flog.Default.Handle(flog.Fmsg("discovered %d upnp devices", len(ds)))
	port := cl.incomingPeerPort()
	cl.unlock()
	for _, d := range ds {
		go addPortMapping(d, upnp.TCP, port, cl.config.Debug)
		go addPortMapping(d, upnp.UDP, port, cl.config.Debug)
	}
	cl.lock()
}
