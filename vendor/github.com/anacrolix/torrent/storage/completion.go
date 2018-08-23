package storage

import (
	"log"

	"github.com/anacrolix/torrent/metainfo"
)

type PieceCompletionGetSetter interface {
	Get(metainfo.PieceKey) (Completion, error)
	Set(_ metainfo.PieceKey, complete bool) error
}

// Implementations track the completion of pieces. It must be concurrent-safe.
type PieceCompletion interface {
	PieceCompletionGetSetter
	Close() error
}

func pieceCompletionForDir(dir string) (ret PieceCompletion) {
	ret, err := NewBoltPieceCompletion(dir)
	if err != nil {
		log.Printf("couldn't open piece completion db in %q: %s", dir, err)
		ret = NewMapPieceCompletion()
	}
	return
}
