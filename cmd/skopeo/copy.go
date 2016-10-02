package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/containers/image/copy"
	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/urfave/cli"
)

// contextsFromGlobalOptions returns source and destionation types.SystemContext depending on c.
func contextsFromGlobalOptions(c *cli.Context) (*types.SystemContext, *types.SystemContext, error) {
	sourceCtx, err := contextFromGlobalOptions(c, "src-")
	if err != nil {
		return nil, nil, err
	}

	destinationCtx, err := contextFromGlobalOptions(c, "dest-")
	if err != nil {
		return nil, nil, err
	}

	return sourceCtx, destinationCtx, nil
}

func copyHandler(context *cli.Context) error {
	if len(context.Args()) != 2 {
		return errors.New("Usage: copy source destination")
	}

	policyContext, err := getPolicyContext(context)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("No system policy found. Install the default-policy.json from the source repo: %v", err)
		}
		return fmt.Errorf("Error loading trust policy: %v", err)
	}
	defer policyContext.Destroy()

	srcRef, err := alltransports.ParseImageName(context.Args()[0])
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", context.Args()[0], err)
	}
	destRef, err := alltransports.ParseImageName(context.Args()[1])
	if err != nil {
		return fmt.Errorf("Invalid destination name %s: %v", context.Args()[1], err)
	}
	signBy := context.String("sign-by")
	removeSignatures := context.Bool("remove-signatures")

	sourceCtx, destinationCtx, err := contextsFromGlobalOptions(context)
	if err != nil {
		return err
	}

	return copy.Image(policyContext, destRef, srcRef, &copy.Options{
		RemoveSignatures: removeSignatures,
		SignBy:           signBy,
		ReportWriter:     os.Stdout,
		SourceCtx:        sourceCtx,
		DestinationCtx:   destinationCtx,
	})
}

var copyCmd = cli.Command{
	Name:  "copy",
	Usage: "Copy an IMAGE-NAME from one location to another",
	Description: fmt.Sprintf(`

	Container "IMAGE-NAME" uses a "transport":"details" format.

	Supported transports:
	%s

	See skopeo(1) section "IMAGE NAMES" for the expected format
	`, strings.Join(transports.ListNames(), ", ")),
	ArgsUsage: "SOURCE-IMAGE DESTINATION-IMAGE",
	Action:    copyHandler,
	// FIXME: Do we need to namespace the GPG aspect?
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "remove-signatures",
			Usage: "Do not copy signatures from SOURCE-IMAGE",
		},
		cli.StringFlag{
			Name:  "sign-by",
			Usage: "Sign the image using a GPG key with the specified `FINGERPRINT`",
		},
		cli.StringFlag{
			Name:  "src-creds, screds",
			Value: "",
			Usage: "Use `USERNAME[:PASSWORD]` for accessing the source registry",
		},
		cli.StringFlag{
			Name:  "dest-creds, dcreds",
			Value: "",
			Usage: "Use `USERNAME[:PASSWORD]` for accessing the destination registry",
		},
		cli.StringFlag{
			Name:  "src-cert-dir",
			Value: "",
			Usage: "use certificates at `PATH` (*.crt, *.cert, *.key) to connect to the source registry",
		},
		cli.BoolTFlag{
			Name:  "src-tls-verify",
			Usage: "require HTTPS and verify certificates when talking to the container source registry (defaults to true)",
		},
		cli.StringFlag{
			Name:  "dest-cert-dir",
			Value: "",
			Usage: "use certificates at `PATH` (*.crt, *.cert, *.key) to connect to the destination registry",
		},
		cli.BoolTFlag{
			Name:  "dest-tls-verify",
			Usage: "require HTTPS and verify certificates when talking to the container destination registry (defaults to true)",
		},
		cli.StringFlag{
			Name:  "dest-ostree-tmp-dir",
			Value: "",
			Usage: "`DIRECTORY` to use for OSTree temporary files",
		},
	},
}
