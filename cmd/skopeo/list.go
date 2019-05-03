package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/urfave/cli"
)

type listOptions struct {
	global      *globalOptions
	image       *imageOptions
	regUsername string
	regPassword string
}

func listCmd(global *globalOptions) cli.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	imageFlags, imageOpts := imageFlags(global, sharedOpts, "", "")
	opts := listOptions{
		global: global,
		image:  imageOpts,
	}
	return cli.Command{
		Name:  "list",
		Usage: "Get list of repos of REGISTRY-NAME",
		Description: fmt.Sprint(`
		Get list of repos of REGISTRY-NAME

		Supports Docker v2 registries only.

		See skopeo(1) section "REGISTRY-NAMES" for the expected format
		`),
		ArgsUsage: "REGISTRY-NAME",
		Before:    needsRexec,
		Action:    commandAction(opts.run),
		// Flags:     append(sharedFlags, imageFlags...),
		Flags: append(append([]cli.Flag{
			cli.StringFlag{
				Name:        "username, u",
				Usage:       "Registry username, defaults to empty string",
				Destination: &opts.regUsername,
			},
			cli.StringFlag{
				Name:        "password, p",
				Usage:       "Registry password, defaults to empty string",
				Destination: &opts.regPassword,
			},
		}, sharedFlags...), imageFlags...),
	}
}

func (opts *listOptions) run(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("Usage: list registryURL")
	}

	reg := NewRegistry(args[0])
	reg.getAllReposWithTags()

	return nil
}
