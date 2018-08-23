package utp

import (
	"log"
	"os"
)

const (
	logCallbacks = false
	utpLogging   = false
)

var Logger = log.New(os.Stderr, "go-libutp: ", log.LstdFlags|log.Lshortfile)
