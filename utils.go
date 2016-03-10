package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/projectatomic/skopeo/docker"
	"strings"
)

func parseImage(c *cli.Context) (image, error) {
	img := c.Args().First()
	switch {
	case strings.HasPrefix(img, docker.DockerPrefix):
		return parseDockerImage(c, strings.TrimPrefix(img, docker.DockerPrefix))
		//case strings.HasPrefix(img, appcPrefix):
		//
	}
	return nil, fmt.Errorf("no valid prefix provided")
}
