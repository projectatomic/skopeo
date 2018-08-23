package krpc

import (
	"fmt"

	"github.com/anacrolix/torrent/bencode"
)

var ErrorMethodUnknown = Error{
	Code: 204,
	Msg:  "Method Unknown",
}

// Represented as a string or list in bencode.
type Error struct {
	Code int
	Msg  string
}

var (
	_ bencode.Unmarshaler = (*Error)(nil)
	_ bencode.Marshaler   = (*Error)(nil)
	_ error               = Error{}
)

func (e *Error) UnmarshalBencode(_b []byte) (err error) {
	var _v interface{}
	err = bencode.Unmarshal(_b, &_v)
	if err != nil {
		return
	}
	switch v := _v.(type) {
	case []interface{}:
		func() {
			defer func() {
				r := recover()
				if r == nil {
					return
				}
				err = fmt.Errorf("unpacking %#v: %s", v, r)
			}()
			e.Code = int(v[0].(int64))
			e.Msg = v[1].(string)
		}()
	case string:
		e.Msg = v
	default:
		err = fmt.Errorf(`KRPC error bencode value has unexpected type: %T`, _v)
	}
	return
}

func (e Error) MarshalBencode() (ret []byte, err error) {
	return bencode.Marshal([]interface{}{e.Code, e.Msg})
}

func (e Error) Error() string {
	return fmt.Sprintf("KRPC error %d: %s", e.Code, e.Msg)
}
