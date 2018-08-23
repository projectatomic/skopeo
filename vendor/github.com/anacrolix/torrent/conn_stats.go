package torrent

import (
	"fmt"
	"io"
	"reflect"
	"sync/atomic"

	pp "github.com/anacrolix/torrent/peer_protocol"
)

// Various connection-level metrics. At the Torrent level these are
// aggregates. Chunks are messages with data payloads. Data is actual torrent
// content without any overhead. Useful is something we needed locally.
// Unwanted is something we didn't ask for (but may still be useful). Written
// is things sent to the peer, and Read is stuff received from them.
type ConnStats struct {
	// Total bytes on the wire. Includes handshakes and encryption.
	BytesWritten     Count
	BytesWrittenData Count

	BytesRead           Count
	BytesReadData       Count
	BytesReadUsefulData Count

	ChunksWritten Count

	ChunksRead       Count
	ChunksReadUseful Count
	ChunksReadWasted Count

	MetadataChunksRead Count

	// Number of pieces data was written to, that subsequently passed verification.
	PiecesDirtiedGood Count
	// Number of pieces data was written to, that subsequently failed
	// verification. Note that a connection may not have been the sole dirtier
	// of a piece.
	PiecesDirtiedBad Count
}

func (me *ConnStats) Copy() (ret ConnStats) {
	for i := 0; i < reflect.TypeOf(ConnStats{}).NumField(); i++ {
		n := reflect.ValueOf(me).Elem().Field(i).Addr().Interface().(*Count).Int64()
		reflect.ValueOf(&ret).Elem().Field(i).Addr().Interface().(*Count).Add(n)
	}
	return
}

type Count struct {
	n int64
}

var _ fmt.Stringer = (*Count)(nil)

func (me *Count) Add(n int64) {
	atomic.AddInt64(&me.n, n)
}

func (me *Count) Int64() int64 {
	return atomic.LoadInt64(&me.n)
}

func (me *Count) String() string {
	return fmt.Sprintf("%v", me.Int64())
}

func (cs *ConnStats) wroteMsg(msg *pp.Message) {
	// TODO: Track messages and not just chunks.
	switch msg.Type {
	case pp.Piece:
		cs.ChunksWritten.Add(1)
		cs.BytesWrittenData.Add(int64(len(msg.Piece)))
	}
}

func (cs *ConnStats) readMsg(msg *pp.Message) {
	// We want to also handle extended metadata pieces here, but we wouldn't
	// have decoded the extended payload yet.
	switch msg.Type {
	case pp.Piece:
		cs.ChunksRead.Add(1)
		cs.BytesReadData.Add(int64(len(msg.Piece)))
	}
}

func (cs *ConnStats) incrementPiecesDirtiedGood() {
	cs.PiecesDirtiedGood.Add(1)
}

func (cs *ConnStats) incrementPiecesDirtiedBad() {
	cs.PiecesDirtiedBad.Add(1)
}

func add(n int64, f func(*ConnStats) *Count) func(*ConnStats) {
	return func(cs *ConnStats) {
		p := f(cs)
		p.Add(n)
	}
}

type connStatsReadWriter struct {
	rw io.ReadWriter
	c  *connection
}

func (me connStatsReadWriter) Write(b []byte) (n int, err error) {
	n, err = me.rw.Write(b)
	me.c.wroteBytes(int64(n))
	return
}

func (me connStatsReadWriter) Read(b []byte) (n int, err error) {
	n, err = me.rw.Read(b)
	me.c.readBytes(int64(n))
	return
}
