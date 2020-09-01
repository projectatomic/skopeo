package main

import (
	"io"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type manifestOptions struct {
}

func manifestCmd() *cobra.Command {
	opts := manifestOptions{}
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Manifest operations",
		Long: `Manifest related commands, pick one of the following actions:
* create
* push

`,
		RunE:    commandAction(opts.run),
		Example: `skopeo manifest <create|push> <sub-command args>`,
	}
	adjustUsage(cmd)
	return cmd
}

func (opts *manifestOptions) run(args []string, stdout io.Writer) error {
	return errors.New("Please use one of the sub-commands: create, push")
}
