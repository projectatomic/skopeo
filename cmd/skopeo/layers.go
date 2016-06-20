package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

// TODO(runcom): document args and usage
var layersCmd = cli.Command{
	Name:  "layers",
	Usage: "get images layers",
	Action: func(c *cli.Context) {
		img, err := parseImage(c)
		if err != nil {
			logrus.Fatal(err)
		}
		if err := img.Layers(c.Args().Tail()...); err != nil {
			logrus.Fatal(err)
		}
	},
}
