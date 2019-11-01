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
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
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
	global *globalOptions
	image  *imageOptions
	raw    bool // Output the raw manifest instead of parsing information about the image
	config bool // Output the raw config blob instead of parsing information about the image
}

func inspectCmd(global *globalOptions) cli.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	imageFlags, imageOpts := imageFlags(global, sharedOpts, "", "")
	opts := inspectOptions{
		global: global,
		image:  imageOpts,
	}
	return cli.Command{
		Name:  "inspect",
		Usage: "Inspect image IMAGE-NAME",
		Description: fmt.Sprintf(`
	Return low-level information about "IMAGE-NAME" in a registry/transport

	Supported transports:
	%s

	See skopeo(1) section "IMAGE NAMES" for the expected format
	`, strings.Join(transports.ListNames(), ", ")),
		ArgsUsage: "IMAGE-NAME",
		Flags: append(append([]cli.Flag{
			cli.BoolFlag{
				Name:        "raw",
				Usage:       "output raw manifest or configuration",
				Destination: &opts.raw,
			},
			cli.BoolFlag{
				Name:        "config",
				Usage:       "output configuration",
				Destination: &opts.config,
			},
		}, sharedFlags...), imageFlags...),
		Action: commandAction(opts.run),
	}
}

func (opts *inspectOptions) run(args []string, stdout io.Writer) (retErr error) {
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

	src, err := parseImageSource(ctx, opts.image, imageName)
	if err != nil {
		return fmt.Errorf("Error parsing image name %q: %v", imageName, err)
	}

	defer func() {
		if err := src.Close(); err != nil {
			retErr = errors.Wrapf(retErr, fmt.Sprintf("(could not close image: %v) ", err))
		}
	}()

	rawManifest, _, err := src.GetManifest(ctx, nil)
	if err != nil {
		return fmt.Errorf("Error retrieving manifest for image: %v", err)
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
		configBlob, err := img.ConfigBlob(ctx)
		if err != nil {
			return fmt.Errorf("Error reading configuration blob: %v", err)
		}
		_, err = stdout.Write(configBlob)
		if err != nil {
			return fmt.Errorf("Error writing configuration blob to standard output: %v", err)
		}
		return nil
	} else if opts.config {
		config, err := img.OCIConfig(ctx)
		if err != nil {
			return fmt.Errorf("Error reading OCI-formatted configuration data: %v", err)
		}
		err = json.NewEncoder(stdout).Encode(config)
		if err != nil {
			return fmt.Errorf("Error writing OCI-formatted configuration data to standard output: %v", err)
		}
		return nil
	}

	imgInspect, err := img.Inspect(ctx)
	if err != nil {
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
			if !strings.Contains(err.Error(), "401") {
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
