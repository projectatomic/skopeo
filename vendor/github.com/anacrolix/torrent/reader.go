package torrent

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"

	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/torrent/peer_protocol"
)

type Reader interface {
	io.Reader
	io.Seeker
	io.Closer
	missinggo.ReadContexter
	SetReadahead(int64)
	SetResponsive()
}

// Piece range by piece index, [begin, end).
type pieceRange struct {
	begin, end pieceIndex
}

// Accesses Torrent data via a Client. Reads block until the data is
// available. Seeks and readahead also drive Client behaviour.
type reader struct {
	t          *Torrent
	responsive bool
	// Adjust the read/seek window to handle Readers locked to File extents
	// and the like.
	offset, length int64
	// Ensure operations that change the position are exclusive, like Read()
	// and Seek().
	opMu sync.Mutex

	// Required when modifying pos and readahead, or reading them without
	// opMu.
	mu        sync.Locker
	pos       int64
	readahead int64
	// The cached piece range this reader wants downloaded. The zero value
	// corresponds to nothing. We cache this so that changes can be detected,
	// and bubbled up to the Torrent only as required.
	pieces pieceRange
}

var _ io.ReadCloser = &reader{}

// Don't wait for pieces to complete and be verified. Read calls return as
// soon as they can when the underlying chunks become available.
func (r *reader) SetResponsive() {
	r.responsive = true
	r.t.cl.event.Broadcast()
}

// Disable responsive mode. TODO: Remove?
func (r *reader) SetNonResponsive() {
	r.responsive = false
	r.t.cl.event.Broadcast()
}

// Configure the number of bytes ahead of a read that should also be
// prioritized in preparation for further reads.
func (r *reader) SetReadahead(readahead int64) {
	r.mu.Lock()
	r.readahead = readahead
	r.mu.Unlock()
	r.t.cl.lock()
	defer r.t.cl.unlock()
	r.posChanged()
}

func (r *reader) readable(off int64) (ret bool) {
	if r.t.closed.IsSet() {
		return true
	}
	req, ok := r.t.offsetRequest(r.torrentOffset(off))
	if !ok {
		panic(off)
	}
	if r.responsive {
		return r.t.haveChunk(req)
	}
	return r.t.pieceComplete(pieceIndex(req.Index))
}

// How many bytes are available to read. Max is the most we could require.
func (r *reader) available(off, max int64) (ret int64) {
	off += r.offset
	for max > 0 {
		req, ok := r.t.offsetRequest(off)
		if !ok {
			break
		}
		if !r.t.haveChunk(req) {
			break
		}
		len1 := int64(req.Length) - (off - r.t.requestOffset(req))
		max -= len1
		ret += len1
		off += len1
	}
	// Ensure that ret hasn't exceeded our original max.
	if max < 0 {
		ret += max
	}
	return
}

func (r *reader) waitReadable(off int64) {
	// We may have been sent back here because we were told we could read but
	// it failed.
	r.t.cl.event.Wait()
}

// Calculates the pieces this reader wants downloaded, ignoring the cached
// value at r.pieces.
func (r *reader) piecesUncached() (ret pieceRange) {
	ra := r.readahead
	if ra < 1 {
		// Needs to be at least 1, because [x, x) means we don't want
		// anything.
		ra = 1
	}
	if ra > r.length-r.pos {
		ra = r.length - r.pos
	}
	ret.begin, ret.end = r.t.byteRegionPieces(r.torrentOffset(r.pos), ra)
	return
}

func (r *reader) Read(b []byte) (n int, err error) {
	return r.ReadContext(context.Background(), b)
}

func (r *reader) ReadContext(ctx context.Context, b []byte) (n int, err error) {
	// This is set under the Client lock if the Context is canceled.
	var ctxErr error
	if ctx.Done() != nil {
		ctx, cancel := context.WithCancel(ctx)
		// Abort the goroutine when the function returns.
		defer cancel()
		go func() {
			<-ctx.Done()
			r.t.cl.lock()
			ctxErr = ctx.Err()
			r.t.tickleReaders()
			r.t.cl.unlock()
		}()
	}
	// Hmmm, if a Read gets stuck, this means you can't change position for
	// other purposes. That seems reasonable, but unusual.
	r.opMu.Lock()
	defer r.opMu.Unlock()
	for len(b) != 0 {
		var n1 int
		n1, err = r.readOnceAt(b, r.pos, &ctxErr)
		if n1 == 0 {
			if err == nil {
				panic("expected error")
			}
			break
		}
		b = b[n1:]
		n += n1
		r.mu.Lock()
		r.pos += int64(n1)
		r.posChanged()
		r.mu.Unlock()
	}
	if r.pos >= r.length {
		err = io.EOF
	} else if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return
}

// Wait until some data should be available to read. Tickles the client if it
// isn't. Returns how much should be readable without blocking.
func (r *reader) waitAvailable(pos, wanted int64, ctxErr *error) (avail int64) {
	r.t.cl.lock()
	defer r.t.cl.unlock()
	for !r.readable(pos) && *ctxErr == nil {
		r.waitReadable(pos)
	}
	return r.available(pos, wanted)
}

func (r *reader) torrentOffset(readerPos int64) int64 {
	return r.offset + readerPos
}

// Performs at most one successful read to torrent storage.
func (r *reader) readOnceAt(b []byte, pos int64, ctxErr *error) (n int, err error) {
	if pos >= r.length {
		err = io.EOF
		return
	}
	for {
		avail := r.waitAvailable(pos, int64(len(b)), ctxErr)
		if avail == 0 {
			if r.t.closed.IsSet() {
				err = errors.New("torrent closed")
				return
			}
			if *ctxErr != nil {
				err = *ctxErr
				return
			}
		}
		pi := peer_protocol.Integer(r.torrentOffset(pos) / r.t.info.PieceLength)
		ip := r.t.info.Piece(int(pi))
		po := r.torrentOffset(pos) % r.t.info.PieceLength
		b1 := missinggo.LimitLen(b, ip.Length()-po, avail)
		n, err = r.t.readAt(b1, r.torrentOffset(pos))
		if n != 0 {
			err = nil
			return
		}
		r.t.cl.lock()
		// TODO: Just reset pieces in the readahead window. This might help
		// prevent thrashing with small caches and file and piece priorities.
		log.Printf("error reading torrent %q piece %d offset %d, %d bytes: %s", r.t, pi, po, len(b1), err)
		r.t.updateAllPieceCompletions()
		r.t.updateAllPiecePriorities()
		r.t.cl.unlock()
	}
}

func (r *reader) Close() error {
	r.t.cl.lock()
	defer r.t.cl.unlock()
	r.t.deleteReader(r)
	return nil
}

func (r *reader) posChanged() {
	to := r.piecesUncached()
	from := r.pieces
	if to == from {
		return
	}
	r.pieces = to
	// log.Printf("reader pos changed %v->%v", from, to)
	r.t.readerPosChanged(from, to)
}

func (r *reader) Seek(off int64, whence int) (ret int64, err error) {
	r.opMu.Lock()
	defer r.opMu.Unlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	switch whence {
	case io.SeekStart:
		r.pos = off
	case io.SeekCurrent:
		r.pos += off
	case io.SeekEnd:
		r.pos = r.length + off
	default:
		err = errors.New("bad whence")
	}
	ret = r.pos

	r.posChanged()
	return
}
