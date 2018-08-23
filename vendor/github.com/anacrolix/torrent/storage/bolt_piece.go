package storage

import (
	"encoding/binary"

	"github.com/anacrolix/missinggo/x"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/boltdb/bolt"
)

type boltDBPiece struct {
	db  *bolt.DB
	p   metainfo.Piece
	ih  metainfo.Hash
	key [24]byte
}

var (
	_             PieceImpl = (*boltDBPiece)(nil)
	dataBucketKey           = []byte("data")
)

func (me *boltDBPiece) pc() PieceCompletionGetSetter {
	return boltPieceCompletion{me.db}
}

func (me *boltDBPiece) pk() metainfo.PieceKey {
	return metainfo.PieceKey{me.ih, me.p.Index()}
}

func (me *boltDBPiece) Completion() Completion {
	c, err := me.pc().Get(me.pk())
	x.Pie(err)
	return c
}

func (me *boltDBPiece) MarkComplete() error {
	return me.pc().Set(me.pk(), true)
}

func (me *boltDBPiece) MarkNotComplete() error {
	return me.pc().Set(me.pk(), false)
}
func (me *boltDBPiece) ReadAt(b []byte, off int64) (n int, err error) {
	err = me.db.View(func(tx *bolt.Tx) error {
		db := tx.Bucket(dataBucketKey)
		if db == nil {
			return nil
		}
		ci := off / chunkSize
		off %= chunkSize
		for len(b) != 0 {
			ck := me.chunkKey(int(ci))
			_b := db.Get(ck[:])
			if len(_b) != chunkSize {
				break
			}
			n1 := copy(b, _b[off:])
			off = 0
			ci++
			b = b[n1:]
			n += n1
		}
		return nil
	})
	return
}

func (me *boltDBPiece) chunkKey(index int) (ret [26]byte) {
	copy(ret[:], me.key[:])
	binary.BigEndian.PutUint16(ret[24:], uint16(index))
	return
}

func (me *boltDBPiece) WriteAt(b []byte, off int64) (n int, err error) {
	err = me.db.Update(func(tx *bolt.Tx) error {
		db, err := tx.CreateBucketIfNotExists(dataBucketKey)
		if err != nil {
			return err
		}
		ci := off / chunkSize
		off %= chunkSize
		for len(b) != 0 {
			_b := make([]byte, chunkSize)
			ck := me.chunkKey(int(ci))
			copy(_b, db.Get(ck[:]))
			n1 := copy(_b[off:], b)
			db.Put(ck[:], _b)
			if n1 > len(b) {
				break
			}
			b = b[n1:]
			off = 0
			ci++
			n += n1
		}
		return nil
	})
	return
}
