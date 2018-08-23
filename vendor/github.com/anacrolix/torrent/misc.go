package torrent

import (
	"errors"
	"net"

	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/torrent/metainfo"
	pp "github.com/anacrolix/torrent/peer_protocol"
	"golang.org/x/time/rate"
)

type chunkSpec struct {
	Begin, Length pp.Integer
}

type request struct {
	Index pp.Integer
	chunkSpec
}

func (r request) ToMsg(mt pp.MessageType) pp.Message {
	return pp.Message{
		Type:   mt,
		Index:  r.Index,
		Begin:  r.Begin,
		Length: r.Length,
	}
}

func newRequest(index, begin, length pp.Integer) request {
	return request{index, chunkSpec{begin, length}}
}

func newRequestFromMessage(msg *pp.Message) request {
	switch msg.Type {
	case pp.Request, pp.Cancel, pp.Reject:
		return newRequest(msg.Index, msg.Begin, msg.Length)
	case pp.Piece:
		return newRequest(msg.Index, msg.Begin, pp.Integer(len(msg.Piece)))
	default:
		panic(msg.Type)
	}
}

// The size in bytes of a metadata extension piece.
func metadataPieceSize(totalSize int, piece int) int {
	ret := totalSize - piece*(1<<14)
	if ret > 1<<14 {
		ret = 1 << 14
	}
	return ret
}

// Return the request that would include the given offset into the torrent data.
func torrentOffsetRequest(torrentLength, pieceSize, chunkSize, offset int64) (
	r request, ok bool) {
	if offset < 0 || offset >= torrentLength {
		return
	}
	r.Index = pp.Integer(offset / pieceSize)
	r.Begin = pp.Integer(offset % pieceSize / chunkSize * chunkSize)
	r.Length = pp.Integer(chunkSize)
	pieceLeft := pp.Integer(pieceSize - int64(r.Begin))
	if r.Length > pieceLeft {
		r.Length = pieceLeft
	}
	torrentLeft := torrentLength - int64(r.Index)*pieceSize - int64(r.Begin)
	if int64(r.Length) > torrentLeft {
		r.Length = pp.Integer(torrentLeft)
	}
	ok = true
	return
}

func torrentRequestOffset(torrentLength, pieceSize int64, r request) (off int64) {
	off = int64(r.Index)*pieceSize + int64(r.Begin)
	if off < 0 || off >= torrentLength {
		panic("invalid request")
	}
	return
}

func validateInfo(info *metainfo.Info) error {
	if len(info.Pieces)%20 != 0 {
		return errors.New("pieces has invalid length")
	}
	if info.PieceLength == 0 {
		if info.TotalLength() != 0 {
			return errors.New("zero piece length")
		}
	} else {
		if int((info.TotalLength()+info.PieceLength-1)/info.PieceLength) != info.NumPieces() {
			return errors.New("piece count and file lengths are at odds")
		}
	}
	return nil
}

func chunkIndexSpec(index pp.Integer, pieceLength, chunkSize pp.Integer) chunkSpec {
	ret := chunkSpec{pp.Integer(index) * chunkSize, chunkSize}
	if ret.Begin+ret.Length > pieceLength {
		ret.Length = pieceLength - ret.Begin
	}
	return ret
}

func connLessTrusted(l, r *connection) bool {
	return l.netGoodPiecesDirtied() < r.netGoodPiecesDirtied()
}

func connIsIpv6(nc interface {
	LocalAddr() net.Addr
}) bool {
	ra := nc.LocalAddr()
	rip := missinggo.AddrIP(ra)
	return rip.To4() == nil && rip.To16() != nil
}

func clamp(min, value, max int64) int64 {
	if min > max {
		panic("harumph")
	}
	if value < min {
		value = min
	}
	if value > max {
		value = max
	}
	return value
}

func max(as ...int64) int64 {
	ret := as[0]
	for _, a := range as[1:] {
		if a > ret {
			ret = a
		}
	}
	return ret
}

func min(as ...int64) int64 {
	ret := as[0]
	for _, a := range as[1:] {
		if a < ret {
			ret = a
		}
	}
	return ret
}

var unlimited = rate.NewLimiter(rate.Inf, 0)

type (
	pieceIndex = int
	InfoHash   = metainfo.Hash
	IpPort     = missinggo.IpPort
)
