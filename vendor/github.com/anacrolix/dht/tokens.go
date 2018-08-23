package dht

import (
	"crypto/sha1"
	"encoding/binary"
	"time"

	"github.com/bradfitz/iter"
)

// Manages creation and validation of tokens issued to querying nodes.
type tokenServer struct {
	// Something only we know that peers can't guess, so they can't deduce valid tokens.
	secret []byte
	// How long between token changes.
	interval time.Duration
	// How many intervals may pass between the current interval, and one used to generate a token before it is invalid.
	maxIntervalDelta int
	timeNow          func() time.Time
}

func (me tokenServer) CreateToken(addr Addr) string {
	return me.createToken(addr, me.getTimeNow())
}

func (me tokenServer) createToken(addr Addr, t time.Time) string {
	h := sha1.New()
	ip := addr.UDPAddr().IP.To16()
	if len(ip) != 16 {
		panic(ip)
	}
	h.Write(ip)
	ti := t.UnixNano() / int64(me.interval)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(ti))
	h.Write(b[:])
	h.Write(me.secret)
	return string(h.Sum(nil))
}

func (me *tokenServer) ValidToken(token string, addr Addr) bool {
	t := me.getTimeNow()
	for range iter.N(me.maxIntervalDelta + 1) {
		if me.createToken(addr, t) == token {
			return true
		}
		t = t.Add(-me.interval)
	}
	return false
}

func (me *tokenServer) getTimeNow() time.Time {
	if me.timeNow == nil {
		return time.Now()
	}
	return me.timeNow()
}
