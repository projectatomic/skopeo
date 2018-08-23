package storage

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/boltdb/bolt"
)

const (
	boltDbCompleteValue   = "c"
	boltDbIncompleteValue = "i"
)

var (
	completionBucketKey = []byte("completion")
)

type boltPieceCompletion struct {
	db *bolt.DB
}

var _ PieceCompletion = (*boltPieceCompletion)(nil)

func NewBoltPieceCompletion(dir string) (ret PieceCompletion, err error) {
	os.MkdirAll(dir, 0770)
	p := filepath.Join(dir, ".torrent.bolt.db")
	db, err := bolt.Open(p, 0660, &bolt.Options{
		Timeout: time.Second,
	})
	if err != nil {
		return
	}
	db.NoSync = true
	ret = &boltPieceCompletion{db}
	return
}

func (me boltPieceCompletion) Get(pk metainfo.PieceKey) (cn Completion, err error) {
	err = me.db.View(func(tx *bolt.Tx) error {
		cb := tx.Bucket(completionBucketKey)
		if cb == nil {
			return nil
		}
		ih := cb.Bucket(pk.InfoHash[:])
		if ih == nil {
			return nil
		}
		var key [4]byte
		binary.BigEndian.PutUint32(key[:], uint32(pk.Index))
		cn.Ok = true
		switch string(ih.Get(key[:])) {
		case boltDbCompleteValue:
			cn.Complete = true
		case boltDbIncompleteValue:
			cn.Complete = false
		default:
			cn.Ok = false
		}
		return nil
	})
	return
}

func (me boltPieceCompletion) Set(pk metainfo.PieceKey, b bool) error {
	return me.db.Update(func(tx *bolt.Tx) error {
		c, err := tx.CreateBucketIfNotExists(completionBucketKey)
		if err != nil {
			return err
		}
		ih, err := c.CreateBucketIfNotExists(pk.InfoHash[:])
		if err != nil {
			return err
		}
		var key [4]byte
		binary.BigEndian.PutUint32(key[:], uint32(pk.Index))
		return ih.Put(key[:], []byte(func() string {
			if b {
				return boltDbCompleteValue
			} else {
				return boltDbIncompleteValue
			}
		}()))
	})
}

func (me *boltPieceCompletion) Close() error {
	return me.db.Close()
}
