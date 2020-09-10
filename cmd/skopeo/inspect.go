package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/containers/skopeo/cmd/skopeo/inspect"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	global    *globalOptions
	image     *imageOptions
	retryOpts *retry.RetryOptions
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

	if err := retry.RetryIfNecessary(ctx, func() error {
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

	if err := retry.RetryIfNecessary(ctx, func() error {
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
		if err := retry.RetryIfNecessary(ctx, func() error {
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
		if err := retry.RetryIfNecessary(ctx, func() error {
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

	var signers []string
	sig, err := img.Signatures(ctx)
	if err != nil {
		return fmt.Errorf("Error reading image signatures: %v", err)
	}
	if len(sig) > 0 {
		mech, err := signature.NewGPGSigningMechanism()
		if err != nil {
			return fmt.Errorf("Error preparing local GPG: %v", err)
		}
		for _, s := range sig {
			_, id, err := mech.UntrustedSignatureContents(s)
			if err == nil {
				_, kID, err := mech.Verify(s)
				if err == nil {
					signers = append(signers, "Key fingerprint: "+kID+" (verified)")
				} else {
					signers = append(signers, "Key ID: "+id+" (unverified)")
				}
			}
		}
	}

	if err := retry.RetryIfNecessary(ctx, func() error {
		imgInspect, err = img.Inspect(ctx)
		return err
	}, opts.retryOpts); err != nil {
		return err
	}

	outputData := inspect.Output{
		Name: "", // Set below if DockerReference() is known
		Tag:  imgInspect.Tag,
		// Digest is set below.
		Signers:       signers,
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
