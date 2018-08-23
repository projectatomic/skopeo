// +build cgo,!disable_libutp

package torrent

import (
	"github.com/anacrolix/go-libutp"
)

func NewUtpSocket(network, addr string, fc firewallCallback) (utpSocket, error) {
	s, err := utp.NewSocket(network, addr)
	if s == nil {
		return nil, err
	}
	if err != nil {
		return s, err
	}
	if fc != nil {
		s.SetFirewallCallback(utp.FirewallCallback(fc))
	}
	return s, err
}
