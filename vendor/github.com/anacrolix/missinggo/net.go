package missinggo

import "strings"

func IsAddrInUse(err error) bool {
	return strings.Contains(err.Error(), "address already in use")
}
