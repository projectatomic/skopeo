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
	version = "0.1.12-dev"
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
	}
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
