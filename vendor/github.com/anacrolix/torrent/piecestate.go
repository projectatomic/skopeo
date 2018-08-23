package torrent

import (
	"github.com/anacrolix/torrent/storage"
)

// The current state of a piece.
type PieceState struct {
	Priority piecePriority
	storage.Completion
	// The piece is being hashed, or is queued for hash.
	Checking bool
	// Some of the piece has been obtained.
	Partial bool
}

// Represents a series of consecutive pieces with the same state.
type PieceStateRun struct {
	PieceState
	Length int // How many consecutive pieces have this state.
}
