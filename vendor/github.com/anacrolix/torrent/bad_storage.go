package torrent

import (
	"errors"
	"math/rand"
	"strings"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

type badStorage struct{}

var _ storage.ClientImpl = badStorage{}

func (bs badStorage) OpenTorrent(*metainfo.Info, metainfo.Hash) (storage.TorrentImpl, error) {
	return bs, nil
}

func (bs badStorage) Close() error {
	return nil
}

func (bs badStorage) Piece(p metainfo.Piece) storage.PieceImpl {
	return badStoragePiece{p}
}

type badStoragePiece struct {
	p metainfo.Piece
}

var _ storage.PieceImpl = badStoragePiece{}

func (p badStoragePiece) WriteAt(b []byte, off int64) (int, error) {
	return 0, nil
}

func (p badStoragePiece) Completion() storage.Completion {
	return storage.Completion{Complete: true, Ok: true}
}

func (p badStoragePiece) MarkComplete() error {
	return errors.New("psyyyyyyyche")
}

func (p badStoragePiece) MarkNotComplete() error {
	return errors.New("psyyyyyyyche")
}

func (p badStoragePiece) randomlyTruncatedDataString() string {
	return "hello, world\n"[:rand.Intn(14)]
}

func (p badStoragePiece) ReadAt(b []byte, off int64) (n int, err error) {
	r := strings.NewReader(p.randomlyTruncatedDataString())
	return r.ReadAt(b, off+p.p.Offset())
}
