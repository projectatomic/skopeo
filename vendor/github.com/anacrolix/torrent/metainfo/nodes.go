package metainfo

import (
	"fmt"
	"net"
	"strconv"

	"github.com/anacrolix/torrent/bencode"
)

type Node string

var (
	_ bencode.Unmarshaler = new(Node)
)

func (n *Node) UnmarshalBencode(b []byte) (err error) {
	var iface interface{}
	err = bencode.Unmarshal(b, &iface)
	if err != nil {
		return
	}
	switch v := iface.(type) {
	case string:
		*n = Node(v)
	case []interface{}:
		func() {
			defer func() {
				r := recover()
				if r != nil {
					err = r.(error)
				}
			}()
			*n = Node(net.JoinHostPort(v[0].(string), strconv.FormatInt(v[1].(int64), 10)))
		}()
	default:
		err = fmt.Errorf("unsupported type: %T", iface)
	}
	return
}
