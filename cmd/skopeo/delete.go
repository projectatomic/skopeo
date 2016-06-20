package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

func deleteHandler(context *cli.Context) {
	if len(context.Args()) != 1 {
		logrus.Fatal("Usage: delete imageReference")
	}

	image, err := parseImageSource(context, context.Args()[0])
	if err != nil {
		logrus.Fatal(err.Error())
	}

	if err := image.Delete(); err != nil {
		logrus.Fatal(err)
	}
}

var deleteCmd = cli.Command{
	Name:   "delete",
	Usage:  "Delete given image",
	Action: deleteHandler,
}
