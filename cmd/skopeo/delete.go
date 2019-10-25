package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/urfave/cli"
)

type deleteOptions struct {
	global *globalOptions
	image  *imageOptions
}

func deleteCmd(global *globalOptions) cli.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	imageFlags, imageOpts := imageFlags(global, sharedOpts, "", "")
	opts := deleteOptions{
		global: global,
		image:  imageOpts,
	}
	return cli.Command{
		Name:  "delete",
		Usage: "Delete image IMAGE-NAME",
		Description: fmt.Sprintf(`
	Delete an "IMAGE_NAME" from a transport

	Supported transports:
	%s

	See skopeo(1) section "IMAGE NAMES" for the expected format
	`, strings.Join(transports.ListNames(), ", ")),
		ArgsUsage: "IMAGE-NAME",
		Action:    commandAction(opts.run),
		Flags:     append(sharedFlags, imageFlags...),
	}
}

func (opts *deleteOptions) run(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("Usage: delete imageReference")
	}
	imageName := args[0]

	if err := reexecIfNecessaryForImages(imageName); err != nil {
		return err
	}

	ref, err := alltransports.ParseImageName(imageName)
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", imageName, err)
	}

	sys, err := opts.image.newSystemContext()
	if err != nil {
		return err
	}

	ctx, cancel := opts.global.commandTimeoutContext()
	defer cancel()
	return ref.DeleteImage(ctx, sys)
}
