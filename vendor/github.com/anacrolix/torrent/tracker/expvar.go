package tracker

import (
	"expvar"
)

var vars = expvar.NewMap("tracker")
