package missinggo

import (
	"net"
	"strconv"
)

type IpPort struct {
	IP   net.IP
	Port uint16
}

func (me IpPort) String() string {
	return net.JoinHostPort(me.IP.String(), strconv.FormatUint(uint64(me.Port), 10))
}

func IpPortFromNetAddr(na net.Addr) IpPort {
	return IpPort{AddrIP(na), uint16(AddrPort(na))}
}
