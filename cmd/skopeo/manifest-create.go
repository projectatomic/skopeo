package main

import (
	"context"
	"fmt"
	"io"

	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type manifestCreateOptions struct {
	global    *globalOptions
	image     *imageOptions
	retryOpts *retry.RetryOptions
}

func manifestCreateCmd(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	imgFlags, imgOpts := imageFlags(global, sharedOpts, "", "")
	retryFlags, retryOpts := retryFlags()
	opts := manifestCreateOptions{
		global:    global,
		image:     imgOpts,
		retryOpts: retryOpts,
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create multi-arch manifest",
		Long: `Create a multi-architecture manifest referencing the specified images.

`,
		RunE:    commandAction(opts.run),
		Example: `skopeo manifest create [IMAGE...]`,
	}
	adjustUsage(cmd)
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedFlags)
	flags.AddFlagSet(&imgFlags)
	flags.AddFlagSet(&retryFlags)
	return cmd
}

func (opts *manifestCreateOptions) run(args []string, stdout io.Writer) (retErr error) {
	ctx, cancel := opts.global.commandTimeoutContext()
	defer cancel()

	if len(args) == 0 {
		return errors.New("Expected one or more arguments")
	}

	for _, img := range args {
		transport := alltransports.TransportFromImageName(img)
		if transport == nil || docker.Transport.Name() != transport.Name() {
			return fmt.Errorf("Invalid transport type for %s, only the '%s://' one is supported",
				img, docker.Transport.Name())
		}
	}

	components := []manifest.Schema2ManifestDescriptor{}

	for _, imageName := range args {
		component, err := opts.getImageSchema2ManifestDescriptor(ctx, imageName)
		if err != nil {
			return errors.Wrapf(
				err,
				"Error fetching details for %s", imageName)
		}
		components = append(components, component)
	}

	schema := manifest.Schema2ListFromComponents(components)
	dump, err := schema.Serialize()
	if err != nil {
		return errors.Wrapf(err, "Cannot serialize final manifest")
	}
	fmt.Printf("%s\n", dump)

	return nil
}

func (opts *manifestCreateOptions) getImageSchema2ManifestDescriptor(ctx context.Context, imageName string) (details manifest.Schema2ManifestDescriptor, retErr error) {
	var (
		err            error
		imgInspect     *types.ImageInspectInfo
		manifestDigest digest.Digest
		rawManifest    []byte
		src            types.ImageSource
	)

	if err := retry.RetryIfNecessary(ctx, func() error {
		src, err = parseImageSource(ctx, opts.image, imageName)
		return err
	}, opts.retryOpts); err != nil {
		return details, errors.Wrapf(err, "Error parsing image name %q", imageName)
	}

	defer func() {
		if err := src.Close(); err != nil {
			retErr = errors.Wrapf(retErr, fmt.Sprintf("(could not close image: %v) ", err))
		}
	}()

	if err := retry.RetryIfNecessary(ctx, func() error {
		rawManifest, _, err = src.GetManifest(ctx, nil)
		return err
	}, opts.retryOpts); err != nil {
		return details, errors.Wrapf(err, "Error retrieving manifest for image")
	}

	sys, err := opts.image.newSystemContext()
	if err != nil {
		return details, err
	}

	img, err := image.FromUnparsedImage(ctx, sys, image.UnparsedInstance(src, nil))
	if err != nil {
		return details, fmt.Errorf("Error parsing manifest for image: %v", err)
	}

	if err := retry.RetryIfNecessary(ctx, func() error {
		imgInspect, err = img.Inspect(ctx)
		return err
	}, opts.retryOpts); err != nil {
		return details, err
	}

	manifestDigest, err = manifest.Digest(rawManifest)
	if err != nil {
		return details, fmt.Errorf("Error computing manifest digest: %v", err)
	}

	return manifest.Schema2ManifestDescriptor{
		manifest.Schema2Descriptor{
			MediaType: manifest.DockerV2Schema2MediaType,
			Size:      int64(len(rawManifest)),
			Digest:    manifestDigest,
		},
		manifest.Schema2PlatformSpec{
			Architecture: imgInspect.Architecture,
			OS:           imgInspect.Os,
		},
	}, nil
}
