package storage

import (
	"sync"

	"github.com/anacrolix/torrent/metainfo"
)

type mapPieceCompletion struct {
	mu sync.Mutex
	m  map[metainfo.PieceKey]bool
}

var _ PieceCompletion = (*mapPieceCompletion)(nil)

func NewMapPieceCompletion() PieceCompletion {
	return &mapPieceCompletion{m: make(map[metainfo.PieceKey]bool)}
}

func (*mapPieceCompletion) Close() error { return nil }

func (me *mapPieceCompletion) Get(pk metainfo.PieceKey) (c Completion, err error) {
	me.mu.Lock()
	defer me.mu.Unlock()
	c.Complete, c.Ok = me.m[pk]
	return
}

func (me *mapPieceCompletion) Set(pk metainfo.PieceKey, b bool) error {
	me.mu.Lock()
	defer me.mu.Unlock()
	if me.m == nil {
		me.m = make(map[metainfo.PieceKey]bool)
	}
	me.m[pk] = b
	return nil
}
