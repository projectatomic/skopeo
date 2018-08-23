package conntrack

import "expvar"

type Protocol = string

type Endpoint = string

type Entry struct {
	Protocol
	LocalAddr  Endpoint
	RemoteAddr Endpoint
}

var expvars = expvar.NewMap("conntrack")
