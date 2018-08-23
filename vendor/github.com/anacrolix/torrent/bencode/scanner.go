package bencode

import (
	"errors"
	"io"
)

// Implements io.ByteScanner over io.Reader, for use in Decoder, to ensure
// that as little as the undecoded input Reader is consumed as possible.
type scanner struct {
	r      io.Reader
	b      [1]byte // Buffer for ReadByte
	unread bool    // True if b has been unread, and so should be returned next
}

func (me *scanner) Read(b []byte) (int, error) {
	return me.r.Read(b)
}

func (me *scanner) ReadByte() (byte, error) {
	if me.unread {
		me.unread = false
		return me.b[0], nil
	}
	n, err := me.r.Read(me.b[:])
	if n == 1 {
		err = nil
	}
	return me.b[0], err
}

func (me *scanner) UnreadByte() error {
	if me.unread {
		return errors.New("byte already unread")
	}
	me.unread = true
	return nil
}
