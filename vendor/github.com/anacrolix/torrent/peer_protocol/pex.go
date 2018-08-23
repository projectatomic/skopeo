package peer_protocol

import "github.com/anacrolix/dht/krpc"

type PexMsg struct {
	Added       krpc.CompactIPv4NodeAddrs `bencode:"added"`
	AddedFlags  []PexPeerFlags            `bencode:"added.f"`
	Added6      krpc.CompactIPv6NodeAddrs `bencode:"added6"`
	Added6Flags []PexPeerFlags            `bencode:"added6.f"`
	Dropped     krpc.CompactIPv4NodeAddrs `bencode:"dropped"`
	Dropped6    krpc.CompactIPv6NodeAddrs `bencode:"dropped6"`
}

type PexPeerFlags byte

func (me PexPeerFlags) Get(f PexPeerFlags) bool {
	return me&f == f
}

const (
	PexPrefersEncryption = 0x01
	PexSeedUploadOnly    = 0x02
	PexSupportsUtp       = 0x04
	PexHolepunchSupport  = 0x08
	PexOutgoingConn      = 0x10
)
