package httptoo

import (
	"net"
	"net/http"

	"github.com/anacrolix/missinggo"
)

// Request is intended for localhost, either with a localhost name, or
// loopback IP.
func RequestIsForLocalhost(r *http.Request) bool {
	hostHost := missinggo.SplitHostMaybePort(r.Host).Host
	if ip := net.ParseIP(hostHost); ip != nil {
		return ip.IsLoopback()
	}
	return hostHost == "localhost"
}

// Request originated from a loopback IP.
func RequestIsFromLocalhost(r *http.Request) bool {
	return net.ParseIP(missinggo.SplitHostMaybePort(r.RemoteAddr).Host).IsLoopback()
}
