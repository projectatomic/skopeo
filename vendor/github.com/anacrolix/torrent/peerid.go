package torrent

// Peer client ID.
type PeerID [20]byte

// // Pretty prints the ID as hex, except parts that adher to the Peer ID
// // Conventions of BEP 20.
// func (me PeerID) String() string {
// 	// if me[0] == '-' && me[7] == '-' {
// 	// 	return string(me[:8]) + hex.EncodeToString(me[8:])
// 	// }
// 	// return hex.EncodeToString(me[:])
// 	return fmt.Sprintf("%+q", me[:])
// }
