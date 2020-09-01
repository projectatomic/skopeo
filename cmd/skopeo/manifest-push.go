package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

type manifestPushOptions struct {
	global    *globalOptions
	image     *imageOptions
	retryOpts *retry.RetryOptions
	input     string
}

func manifestPushCmd(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	imgFlags, imgOpts := imageFlags(global, sharedOpts, "", "")
	retryFlags, retryOpts := retryFlags()
	opts := manifestPushOptions{
		global:    global,
		image:     imgOpts,
		retryOpts: retryOpts,
	}
	cmd := &cobra.Command{
		Use:     "push",
		Short:   "Push multi-arch manifest",
		Long:    `Push a multi-architecture manifest to the specified container registry.`,
		RunE:    commandAction(opts.run),
		Example: `skopeo manifest push MANIFEST`,
	}
	adjustUsage(cmd)
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedFlags)
	flags.AddFlagSet(&imgFlags)
	flags.AddFlagSet(&retryFlags)
	flags.StringVarP(&opts.input, "input", "i", "", "Read from specified file (default stdin)")
	return cmd
}

func (opts *manifestPushOptions) run(args []string, stdout io.Writer) (retErr error) {
	var (
		manifest []byte
		err      error
	)

	ctx, cancel := opts.global.commandTimeoutContext()
	defer cancel()

	if len(args) != 1 {
		return errors.New("Expected one argument")
	}

	if len(opts.input) > 0 {
		manifest, err = ioutil.ReadFile(opts.input)
		if err != nil {
			return errors.Wrapf(err, "Cannot read manifest from file %s", opts.input)
		}
	} else {
		if terminal.IsTerminal(int(os.Stdin.Fd())) {
			return errors.Errorf("cannot read from terminal. Use command-line redirection or the --input flag.")
		}

		var buff bytes.Buffer
		bw := io.Writer(&buff)
		_, err := io.Copy(bw, os.Stdin)
		if err != nil {
			return errors.Wrapf(err, "Cannot read manifest from stdin")
		}
		manifest = buff.Bytes()
	}

	destName := args[0]
	transport := alltransports.TransportFromImageName(destName)
	if transport == nil || docker.Transport.Name() != transport.Name() {
		return fmt.Errorf("Invalid transport type for %s, only the '%s://' one is supported",
			destName, docker.Transport.Name())
	}

	destRef, err := alltransports.ParseImageName(destName)
	if err != nil {
		return fmt.Errorf("Invalid destination name %s: %v", destName, err)
	}

	destinationCtx, err := opts.image.newSystemContext()
	if err != nil {
		return err
	}

	dest, err := destRef.NewImageDestination(ctx, destinationCtx)
	if err != nil {
		return errors.Wrapf(err, "Error initializing destination %s", transports.ImageName(destRef))
	}
	defer func() {
		if err := dest.Close(); err != nil {
			retErr = errors.Wrapf(retErr, " (dest: %v)", err)
		}
	}()

	fmt.Println("Pushing manifest")
	if err = dest.PutManifest(ctx, manifest, nil); err != nil {
		return errors.Wrapf(err, "Error while pushing manifest")
	}
	fmt.Println("Manifest successfully pushed")

	return nil
}
