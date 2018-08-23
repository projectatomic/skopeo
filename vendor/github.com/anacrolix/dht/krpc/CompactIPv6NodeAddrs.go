package krpc

import "github.com/anacrolix/missinggo/slices"

type CompactIPv6NodeAddrs []NodeAddr

func (CompactIPv6NodeAddrs) ElemSize() int { return 18 }

func (me CompactIPv6NodeAddrs) MarshalBinary() ([]byte, error) {
	return marshalBinarySlice(slices.Map(func(na NodeAddr) NodeAddr {
		na.IP = na.IP.To16()
		return na
	}, me).(CompactIPv6NodeAddrs))
}

func (me CompactIPv6NodeAddrs) MarshalBencode() ([]byte, error) {
	return bencodeBytesResult(me.MarshalBinary())
}

func (me *CompactIPv6NodeAddrs) UnmarshalBinary(b []byte) error {
	return unmarshalBinarySlice(me, b)
}

func (me *CompactIPv6NodeAddrs) UnmarshalBencode(b []byte) error {
	return unmarshalBencodedBinary(me, b)
}

func (me CompactIPv6NodeAddrs) NodeAddrs() []NodeAddr {
	return me
}
