package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// inspectOutput is the output format of (skopeo inspect), primarily so that we can format it with a simple json.MarshalIndent.
type inspectOutput struct {
	Name          string `json:",omitempty"`
	Tag           string `json:",omitempty"`
	Digest        digest.Digest
	RepoTags      []string
	Created       *time.Time
	DockerVersion string
	Labels        map[string]string
	Architecture  string
	Os            string
	Layers        []string
	Env           []string
}

type inspectOptions struct {
	global    *globalOptions
	image     *imageOptions
	retryOpts *retryOptions
	raw       bool // Output the raw manifest instead of parsing information about the image
	config    bool // Output the raw config blob instead of parsing information about the image
}

func inspectCmd(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	imageFlags, imageOpts := imageFlags(global, sharedOpts, "", "")
	retryFlags, retryOpts := retryFlags()
	opts := inspectOptions{
		global:    global,
		image:     imageOpts,
		retryOpts: retryOpts,
	}
	cmd := &cobra.Command{
		Use:   "inspect [command options] IMAGE-NAME",
		Short: "Inspect image IMAGE-NAME",
		Long: fmt.Sprintf(`Return low-level information about "IMAGE-NAME" in a registry/transport
Supported transports:
%s

See skopeo(1) section "IMAGE NAMES" for the expected format
`, strings.Join(transports.ListNames(), ", ")),
		RunE:    commandAction(opts.run),
		Example: `skopeo inspect docker://docker.io/fedora`,
	}
	adjustUsage(cmd)
	flags := cmd.Flags()
	flags.BoolVar(&opts.raw, "raw", false, "output raw manifest or configuration")
	flags.BoolVar(&opts.config, "config", false, "output configuration")
	flags.AddFlagSet(&sharedFlags)
	flags.AddFlagSet(&imageFlags)
	flags.AddFlagSet(&retryFlags)
	return cmd
}

func (opts *inspectOptions) run(args []string, stdout io.Writer) (retErr error) {
	var (
		rawManifest []byte
		src         types.ImageSource
		imgInspect  *types.ImageInspectInfo
	)
	ctx, cancel := opts.global.commandTimeoutContext()
	defer cancel()

	if len(args) != 1 {
		return errors.New("Exactly one argument expected")
	}
	imageName := args[0]

	if err := reexecIfNecessaryForImages(imageName); err != nil {
		return err
	}

	sys, err := opts.image.newSystemContext()
	if err != nil {
		return err
	}

	if err := retryIfNecessary(ctx, func() error {
		src, err = parseImageSource(ctx, opts.image, imageName)
		return err
	}, opts.retryOpts); err != nil {
		return errors.Wrapf(err, "Error parsing image name %q", imageName)
	}

	defer func() {
		if err := src.Close(); err != nil {
			retErr = errors.Wrapf(retErr, fmt.Sprintf("(could not close image: %v) ", err))
		}
	}()

	if err := retryIfNecessary(ctx, func() error {
		rawManifest, _, err = src.GetManifest(ctx, nil)
		return err
	}, opts.retryOpts); err != nil {
		return errors.Wrapf(err, "Error retrieving manifest for image")
	}

	if opts.raw && !opts.config {
		_, err := stdout.Write(rawManifest)
		if err != nil {
			return fmt.Errorf("Error writing manifest to standard output: %v", err)
		}
		return nil
	}

	img, err := image.FromUnparsedImage(ctx, sys, image.UnparsedInstance(src, nil))
	if err != nil {
		return fmt.Errorf("Error parsing manifest for image: %v", err)
	}

	if opts.config && opts.raw {
		var configBlob []byte
		if err := retryIfNecessary(ctx, func() error {
			configBlob, err = img.ConfigBlob(ctx)
			return err
		}, opts.retryOpts); err != nil {
			return errors.Wrapf(err, "Error reading configuration blob")
		}
		_, err = stdout.Write(configBlob)
		if err != nil {
			return fmt.Errorf("Error writing configuration blob to standard output: %v", err)
		}
		return nil
	} else if opts.config {
		var config *v1.Image
		if err := retryIfNecessary(ctx, func() error {
			config, err = img.OCIConfig(ctx)
			return err
		}, opts.retryOpts); err != nil {
			return errors.Wrapf(err, "Error reading OCI-formatted configuration data")
		}
		err = json.NewEncoder(stdout).Encode(config)
		if err != nil {
			return fmt.Errorf("Error writing OCI-formatted configuration data to standard output: %v", err)
		}
		return nil
	}

	if err := retryIfNecessary(ctx, func() error {
		imgInspect, err = img.Inspect(ctx)
		return err
	}, opts.retryOpts); err != nil {
		return err
	}

	outputData := inspectOutput{
		Name: "", // Set below if DockerReference() is known
		Tag:  imgInspect.Tag,
		// Digest is set below.
		RepoTags:      []string{}, // Possibly overriden for docker.Transport.
		Created:       imgInspect.Created,
		DockerVersion: imgInspect.DockerVersion,
		Labels:        imgInspect.Labels,
		Architecture:  imgInspect.Architecture,
		Os:            imgInspect.Os,
		Layers:        imgInspect.Layers,
		Env:           imgInspect.Env,
	}
	outputData.Digest, err = manifest.Digest(rawManifest)
	if err != nil {
		return fmt.Errorf("Error computing manifest digest: %v", err)
	}
	if dockerRef := img.Reference().DockerReference(); dockerRef != nil {
		outputData.Name = dockerRef.Name()
	}
	if img.Reference().Transport() == docker.Transport {
		sys, err := opts.image.newSystemContext()
		if err != nil {
			return err
		}
		outputData.RepoTags, err = docker.GetRepositoryTags(ctx, sys, img.Reference())
		if err != nil {
			// some registries may decide to block the "list all tags" endpoint
			// gracefully allow the inspect to continue in this case. Currently
			// the IBM Bluemix container registry has this restriction.
			// In addition, AWS ECR rejects it with 403 (Forbidden) if the "ecr:ListImages"
			// action is not allowed.
			if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "403") {
				return fmt.Errorf("Error determining repository tags: %v", err)
			}
			logrus.Warnf("Registry disallows tag list retrieval; skipping")
		}
	}
	out, err := json.MarshalIndent(outputData, "", "    ")
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s\n", string(out))
	return nil
}
