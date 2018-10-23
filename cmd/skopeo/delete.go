package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/urfave/cli"
)

func deleteHandler(c *cli.Context) error {
	if len(c.Args()) != 1 {
		return errors.New("Usage: delete imageReference")
	}

	ref, err := alltransports.ParseImageName(c.Args()[0])
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", c.Args()[0], err)
	}

	sys, err := contextFromGlobalOptions(c, "")
	if err != nil {
		return err
	}
	return ref.DeleteImage(context.Background(), sys)
}

var deleteCmd = cli.Command{
	Name:  "delete",
	Usage: "Delete image IMAGE-NAME",
	Description: fmt.Sprintf(`
	Delete an "IMAGE_NAME" from a transport

	Supported transports:
	%s

	See skopeo(1) section "IMAGE NAMES" for the expected format
	`, strings.Join(transports.ListNames(), ", ")),
	ArgsUsage: "IMAGE-NAME",
	Action:    deleteHandler,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "authfile",
			Usage: "path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json",
		},
		cli.StringFlag{
			Name:  "creds",
			Value: "",
			Usage: "Use `USERNAME[:PASSWORD]` for accessing the registry",
		},
		cli.BoolFlag{
			Name:  "no-creds",
			Usage: "access the registry anonymously",
		},
		cli.StringFlag{
			Name:  "cert-dir",
			Value: "",
			Usage: "use certificates at `PATH` (*.crt, *.cert, *.key) to connect to the registry",
		},
		cli.BoolTFlag{
			Name:  "tls-verify",
			Usage: "require HTTPS and verify certificates when talking to container registries (defaults to true)",
		},
	},
}
