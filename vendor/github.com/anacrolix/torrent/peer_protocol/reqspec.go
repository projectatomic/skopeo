package peer_protocol

import "fmt"

type RequestSpec struct {
	Index, Begin, Length Integer
}

func (me RequestSpec) String() string {
	return fmt.Sprintf("{%d %d %d}", me.Index, me.Begin, me.Length)
}
