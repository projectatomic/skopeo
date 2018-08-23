package torrent

import (
	"crypto"
	"expvar"

	pp "github.com/anacrolix/torrent/peer_protocol"
)

const (
	pieceHash        = crypto.SHA1
	maxRequests      = 250    // Maximum pending requests we allow peers to send us.
	defaultChunkSize = 0x4000 // 16KiB
)

// These are our extended message IDs. Peers will use these values to
// select which extension a message is intended for.
const (
	metadataExtendedId = iota + 1 // 0 is reserved for deleting keys
	pexExtendedId
)

func defaultPeerExtensionBytes() PeerExtensionBits {
	return pp.NewPeerExtensionBytes(pp.ExtensionBitDHT, pp.ExtensionBitExtended, pp.ExtensionBitFast)
}

// I could move a lot of these counters to their own file, but I suspect they
// may be attached to a Client someday.
var (
	torrent = expvar.NewMap("torrent")

	peersAddedBySource = expvar.NewMap("peersAddedBySource")

	pieceHashedCorrect    = expvar.NewInt("pieceHashedCorrect")
	pieceHashedNotCorrect = expvar.NewInt("pieceHashedNotCorrect")

	completedHandshakeConnectionFlags = expvar.NewMap("completedHandshakeConnectionFlags")
	// Count of connections to peer with same client ID.
	connsToSelf        = expvar.NewInt("connsToSelf")
	receivedKeepalives = expvar.NewInt("receivedKeepalives")
	postedKeepalives   = expvar.NewInt("postedKeepalives")
	// Requests received for pieces we don't have.
	requestsReceivedForMissingPieces = expvar.NewInt("requestsReceivedForMissingPieces")
	requestedChunkLengths            = expvar.NewMap("requestedChunkLengths")

	messageTypesReceived = expvar.NewMap("messageTypesReceived")

	// Track the effectiveness of Torrent.connPieceInclinationPool.
	pieceInclinationsReused = expvar.NewInt("pieceInclinationsReused")
	pieceInclinationsNew    = expvar.NewInt("pieceInclinationsNew")
	pieceInclinationsPut    = expvar.NewInt("pieceInclinationsPut")
)
