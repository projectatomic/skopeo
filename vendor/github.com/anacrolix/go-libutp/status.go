package utp

import (
	"fmt"
	"io"
)

func WriteStatus(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	for _, s := range libContextToSocket {
		fmt.Fprintf(w, "listening at %s\n", s.Addr())
		fmt.Fprintf(w, "has %d conns\n", len(s.conns))
		fmt.Fprintf(w, "backlog: %d\n", len(s.backlog))
		fmt.Fprintf(w, "\n")
	}
}
