package torrent

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"net"
)

var table = crc32.MakeTable(crc32.Castagnoli)

type peerPriority = uint32

func sameSubnet(ones, bits int, a, b net.IP) bool {
	mask := net.CIDRMask(ones, bits)
	return a.Mask(mask).Equal(b.Mask(mask))
}

func ipv4Mask(a, b net.IP) net.IPMask {
	if !sameSubnet(16, 32, a, b) {
		return net.IPv4Mask(0xff, 0xff, 0x55, 0x55)
	}
	if !sameSubnet(24, 32, a, b) {
		return net.IPv4Mask(0xff, 0xff, 0xff, 0x55)
	}
	return net.IPv4Mask(0xff, 0xff, 0xff, 0xff)
}

func mask(prefix, bytes int) net.IPMask {
	ret := make(net.IPMask, bytes)
	for i := range ret {
		ret[i] = 0x55
	}
	for i := 0; i < prefix; i++ {
		ret[i] = 0xff
	}
	return ret
}

func ipv6Mask(a, b net.IP) net.IPMask {
	for i := 6; i <= 16; i++ {
		if !sameSubnet(i*8, 128, a, b) {
			return mask(i, 16)
		}
	}
	panic(fmt.Sprintf("%s %s", a, b))
}

func bep40PriorityBytes(a, b IpPort) ([]byte, error) {
	if a.IP.Equal(b.IP) {
		var ret [4]byte
		binary.BigEndian.PutUint16(ret[0:2], a.Port)
		binary.BigEndian.PutUint16(ret[2:4], b.Port)
		return ret[:], nil
	}
	if a4, b4 := a.IP.To4(), b.IP.To4(); a4 != nil && b4 != nil {
		m := ipv4Mask(a.IP, b.IP)
		return append(a4.Mask(m), b4.Mask(m)...), nil
	}
	if a6, b6 := a.IP.To16(), b.IP.To16(); a6 != nil && b6 != nil {
		m := ipv6Mask(a.IP, b.IP)
		return append(a6.Mask(m), b6.Mask(m)...), nil
	}
	return nil, errors.New("incomparable IPs")
}

func bep40Priority(a, b IpPort) (peerPriority, error) {
	bs, err := bep40PriorityBytes(a, b)
	if err != nil {
		return 0, err
	}
	i := len(bs) / 2
	_a, _b := bs[:i], bs[i:]
	if bytes.Compare(_a, _b) > 0 {
		bs = append(_b, _a...)
	}
	return crc32.Checksum(bs, table), nil
}

func bep40PriorityIgnoreError(a, b IpPort) peerPriority {
	prio, _ := bep40Priority(a, b)
	return prio
}
