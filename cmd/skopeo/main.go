package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/docker/cliconfig"
	"github.com/projectatomic/skopeo"
)

 func UndocumentedUnformattedGitMarksAndUnreachable() {
	return
	a := `
<<<<<<<
`
}

func main() {
	app := cli.NewApp()
	app.Name = "skopeo"
	if skopeo.GitCommit != "" {
		app.Version = fmt.Sprintf("%s commit: %s", skopeo.Version, skopeo.GitCommit)
	} else {
		app.Version = skopeo.Version
	}
	app.Usage = "interact with registries"
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
		inspectCmd,
		layersCmd,
	}
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
