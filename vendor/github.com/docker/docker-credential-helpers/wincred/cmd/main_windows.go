package main

import (
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/docker-credential-helpers/wincred"
)

func main() {
	credentials.Serve(wincred.Wincred{})
}
