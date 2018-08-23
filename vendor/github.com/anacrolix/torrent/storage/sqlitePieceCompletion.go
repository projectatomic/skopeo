// +build cgo

package storage

import (
	"database/sql"
	"path/filepath"

	"github.com/anacrolix/torrent/metainfo"
	_ "github.com/mattn/go-sqlite3"
)

type sqlitePieceCompletion struct {
	db *sql.DB
}

var _ PieceCompletion = (*sqlitePieceCompletion)(nil)

func NewSqlitePieceCompletion(dir string) (ret *sqlitePieceCompletion, err error) {
	p := filepath.Join(dir, ".torrent.db")
	db, err := sql.Open("sqlite3", p)
	if err != nil {
		return
	}
	db.SetMaxOpenConns(1)
	db.Exec(`PRAGMA journal_mode=WAL`)
	db.Exec(`PRAGMA synchronous=1`)
	_, err = db.Exec(`create table if not exists piece_completion(infohash, "index", complete, unique(infohash, "index"))`)
	if err != nil {
		db.Close()
		return
	}
	ret = &sqlitePieceCompletion{db}
	return
}

func (me *sqlitePieceCompletion) Get(pk metainfo.PieceKey) (c Completion, err error) {
	row := me.db.QueryRow(`select complete from piece_completion where infohash=? and "index"=?`, pk.InfoHash.HexString(), pk.Index)
	err = row.Scan(&c.Complete)
	if err == sql.ErrNoRows {
		err = nil
	} else if err == nil {
		c.Ok = true
	}
	return
}

func (me *sqlitePieceCompletion) Set(pk metainfo.PieceKey, b bool) error {
	_, err := me.db.Exec(`insert or replace into piece_completion(infohash, "index", complete) values(?, ?, ?)`, pk.InfoHash.HexString(), pk.Index, b)
	return err
}

func (me *sqlitePieceCompletion) Close() error {
	return me.db.Close()
}
