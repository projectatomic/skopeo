package main

import (
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/docker-credential-helpers/secretservice"
)

func main() {
	credentials.Serve(secretservice.Secretservice{})
}
