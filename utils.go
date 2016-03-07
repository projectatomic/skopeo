package main

import (
	"fmt"
	"strings"
	"github.com/codegangsta/cli"
)

func parseImage(c *cli.Context) (Image, error) {
	img := c.Args().First()
	switch {
	case strings.HasPrefix(img, dockerPrefix):
		return parseDockerImage(c, strings.TrimPrefix(img, dockerPrefix))
		//case strings.HasPrefix(img, appcPrefix):
		//
	}
	return nil, fmt.Errorf("no valid prefix provided")
}
