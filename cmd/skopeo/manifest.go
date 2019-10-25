package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/containers/image/v5/manifest"
	"github.com/urfave/cli"
)

type manifestDigestOptions struct {
}

func manifestDigestCmd() cli.Command {
	opts := manifestDigestOptions{}
	return cli.Command{
		Name:      "manifest-digest",
		Usage:     "Compute a manifest digest of a file",
		ArgsUsage: "MANIFEST",
		Action:    commandAction(opts.run),
	}
}

func (opts *manifestDigestOptions) run(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("Usage: skopeo manifest-digest manifest")
	}
	manifestPath := args[0]

	man, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("Error reading manifest from %s: %v", manifestPath, err)
	}
	digest, err := manifest.Digest(man)
	if err != nil {
		return fmt.Errorf("Error computing digest: %v", err)
	}
	fmt.Fprintf(stdout, "%s\n", digest)
	return nil
}
