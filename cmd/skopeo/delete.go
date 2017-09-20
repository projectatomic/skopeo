package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/urfave/cli"
)

func deleteHandler(context *cli.Context) error {
	if len(context.Args()) != 1 {
		return errors.New("Usage: delete imageReference")
	}

	ref, err := alltransports.ParseImageName(context.Args()[0])
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", context.Args()[0], err)
	}

	ctx, err := contextFromGlobalOptions(context, "")
	if err != nil {
		return err
	}
	return ref.DeleteImage(ctx)
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
			Name:  "creds",
			Value: "",
			Usage: "Use `USERNAME[:PASSWORD]` for accessing the registry",
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
