package torrent

import "strings"

func LoopbackListenHost(network string) string {
	if strings.Contains(network, "4") {
		return "127.0.0.1"
	} else {
		return "::1"
	}
}
