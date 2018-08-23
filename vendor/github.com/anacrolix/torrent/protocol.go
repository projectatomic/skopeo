package torrent

import (
	pp "github.com/anacrolix/torrent/peer_protocol"
)

func makeCancelMessage(r request) pp.Message {
	return pp.MakeCancelMessage(r.Index, r.Begin, r.Length)
}
