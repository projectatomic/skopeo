package krpc

type CompactIPv4NodeAddrs []NodeAddr

func (CompactIPv4NodeAddrs) ElemSize() int { return 6 }

func (me CompactIPv4NodeAddrs) MarshalBinary() ([]byte, error) {
	return marshalBinarySlice(me)
}

func (me CompactIPv4NodeAddrs) MarshalBencode() ([]byte, error) {
	return bencodeBytesResult(me.MarshalBinary())
}

func (me *CompactIPv4NodeAddrs) UnmarshalBinary(b []byte) error {
	return unmarshalBinarySlice(me, b)
}

func (me *CompactIPv4NodeAddrs) UnmarshalBencode(b []byte) error {
	return unmarshalBencodedBinary(me, b)
}

func (me CompactIPv4NodeAddrs) NodeAddrs() []NodeAddr {
	return me
}
