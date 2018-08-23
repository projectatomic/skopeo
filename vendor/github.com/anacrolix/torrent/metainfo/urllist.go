package metainfo

import (
	"github.com/anacrolix/torrent/bencode"
)

type UrlList []string

var (
	_ bencode.Unmarshaler = (*UrlList)(nil)
)

func (me *UrlList) UnmarshalBencode(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if b[0] == 'l' {
		var l []string
		err := bencode.Unmarshal(b, &l)
		*me = l
		return err
	}
	var s string
	err := bencode.Unmarshal(b, &s)
	*me = []string{s}
	return err
}
