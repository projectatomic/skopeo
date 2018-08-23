package storage

import (
	"io"

	"github.com/anacrolix/torrent/metainfo"
)

// Represents data storage for an unspecified torrent.
type ClientImpl interface {
	OpenTorrent(info *metainfo.Info, infoHash metainfo.Hash) (TorrentImpl, error)
	Close() error
}

// Data storage bound to a torrent.
type TorrentImpl interface {
	Piece(metainfo.Piece) PieceImpl
	Close() error
}

// Interacts with torrent piece data.
type PieceImpl interface {
	// These interfaces are not as strict as normally required. They can
	// assume that the parameters are appropriate for the dimensions of the
	// piece.
	io.ReaderAt
	io.WriterAt
	// Called when the client believes the piece data will pass a hash check.
	// The storage can move or mark the piece data as read-only as it sees
	// fit.
	MarkComplete() error
	MarkNotComplete() error
	// Returns true if the piece is complete.
	Completion() Completion
}

type Completion struct {
	Complete bool
	Ok       bool
}
