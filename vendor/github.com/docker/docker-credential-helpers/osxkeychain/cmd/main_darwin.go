package main

import (
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/docker-credential-helpers/osxkeychain"
)

func main() {
	credentials.Serve(osxkeychain.Osxkeychain{})
}
