package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/projectatomic/skopeo/version"
	"github.com/urfave/cli"
)

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile
var gitCommit = ""

const (
	usage = `interact with registries`
)

func main() {
	app := cli.NewApp()
	app.Name = "skopeo"
	if gitCommit != "" {
		app.Version = fmt.Sprintf("%s commit: %s", version.Version, gitCommit)
	} else {
		app.Version = version.Version
	}
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
			Name:  "cert-path",
			Value: "",
			Usage: "Certificates path to connect to the given registry (cert.pem, key.pem)",
		},
		cli.BoolFlag{
			Name:  "tls-verify",
			Usage: "Whether to verify certificates or not",
		},
	}
	app.Before = func(c *cli.Context) error {
		if c.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Commands = []cli.Command{
		copyCmd,
		inspectCmd,
		layersCmd,
		deleteCmd,
		standaloneSignCmd,
		standaloneVerifyCmd,
	}
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
