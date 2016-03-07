package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/docker/cliconfig"
)

const (
	version = "0.1.10-dev"
	usage   = "inspect images on a registry"
)

var inspectCommand = cli.Command{
	Name:      "inspect",
	Usage:     "",
	Action: func(context *cli.Context) {
		imgInspect, err := inspect(context)
		if err != nil {
			logrus.Fatal(err)
		}
		out, err := json.Marshal(imgInspect)
		if err != nil {
			logrus.Fatal(err)
		}
		fmt.Println(string(out))
	},
	Flags: 	[]cli.Flag{
		cli.StringFlag{
			Name:  "username",
			Value: "",
			Usage: "registry username",
		},
		cli.StringFlag{
			Name:  "password",
			Value: "",
			Usage: "registry password",
		},
		cli.StringFlag{
			Name:  "docker-cfg",
			Value: cliconfig.ConfigDir(),
			Usage: "Docker's cli config for auth",
		},
	},

}

type Kind int

const (
	KindUnknown Kind = iota
	KindDocker
	KindAppc
)

type Image interface {
	Kind() Kind
	GetLayers(layers []string) error
	GetRawManifest(version string) ([]byte, error)
}

// TODO(runcom): document args and usage
var layersCommand = cli.Command{
	Name:      "layers",
	Usage:     "",
	Action: func(context *cli.Context) {
		img, err := parseImage(context)
		if err != nil {
			logrus.Fatal(err)
		}
		if err := img.GetLayers(context.Args().Tail()); err != nil {
			logrus.Fatal(err)
		}
	},
}

func main() {
	app := cli.NewApp()
	app.Name = "skopeo"
	app.Version = version
	app.Usage = usage
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output",
		},
		cli.StringFlag{
			Name:  "username",
			Value: "",
			Usage: "registry username",
		},
		cli.StringFlag{
			Name:  "password",
			Value: "",
			Usage: "registry password",
		},
		cli.StringFlag{
			Name:  "docker-cfg",
			Value: cliconfig.ConfigDir(),
			Usage: "Docker's cli config for auth",
		},
	}
	app.Before = func(c *cli.Context) error {
		if c.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Commands = []cli.Command{
		inspectCommand,
		layersCommand,
	}
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
