package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/docker/cliconfig"
)

const (
	version = "0.1.12-dev"
	usage   = "inspect images on a registry"
)

var inspectCommand = cli.Command{
	Name:  "inspect",
	Usage: "",
	Action: func(context *cli.Context) {
		img, err := parseImage(context)
		if err != nil {
			logrus.Fatal(err)
		}
		if context.Bool("raw") {
			rawManifest, err := img.GetRawManifest("2-1")
			if err != nil {
				logrus.Fatal(err)
			}
			fmt.Println(string(rawManifest))
		} else {
			imgInspect, err := img.GetManifest()
			//imgInspect, err := inspect(context)
			if err != nil {
				logrus.Fatal(err)
			}
			fmt.Println(string(imgInspect))
		}
	},
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "raw",
			Usage: "output raw manifest",
		},
	},
}

type kind int

const (
	kindUnknown kind = iota
	kindDocker
	kindAppc
)

type image interface {
	Kind() kind
	GetLayers(layers []string) error
	GetManifest() ([]byte, error)
	GetRawManifest(version string) ([]byte, error)
}

// TODO(runcom): document args and usage
var layersCommand = cli.Command{
	Name:  "layers",
	Usage: "",
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
	// TODO(runcom)
	//app.EnableBashCompletion = true
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
