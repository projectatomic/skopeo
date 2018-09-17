package main

import (
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/docker-credential-helpers/pass"
)

func main() {
	credentials.Serve(pass.Pass{})
}
