package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
)

type copyOptions struct {
	global            *globalOptions
	srcImage          *imageOptions
	destImage         *imageDestOptions
	additionalTags    cli.StringSlice // For docker-archive: destinations, in addition to the name:tag specified as destination, also add these
	removeSignatures  bool            // Do not copy signatures from the source image
	signByFingerprint string          // Sign the image using a GPG key with the specified fingerprint
	format            optionalString  // Force conversion of the image to a specified format

	dockerTryTorrent      bool
	dockerTorrentTrackers cli.StringSlice
}

func copyCmd(global *globalOptions) cli.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	srcFlags, srcOpts := imageFlags(global, sharedOpts, "src-", "screds")
	destFlags, destOpts := imageDestFlags(global, sharedOpts, "dest-", "dcreds")
	opts := copyOptions{global: global,
		srcImage:  srcOpts,
		destImage: destOpts,
	}

	return cli.Command{
		Name:  "copy",
		Usage: "Copy an IMAGE-NAME from one location to another",
		Description: fmt.Sprintf(`

	Container "IMAGE-NAME" uses a "transport":"details" format.

	Supported transports:
	%s

	See skopeo(1) section "IMAGE NAMES" for the expected format
	`, strings.Join(transports.ListNames(), ", ")),
		ArgsUsage: "SOURCE-IMAGE DESTINATION-IMAGE",
		Action:    commandAction(opts.run),
		// FIXME: Do we need to namespace the GPG aspect?
		Flags: append(append(append([]cli.Flag{
			cli.StringSliceFlag{
				Name:  "additional-tag",
				Usage: "additional tags (supports docker-archive)",
				Value: &opts.additionalTags, // Surprisingly StringSliceFlag does not support Destination:, but modifies Value: in place.
			},
			cli.BoolFlag{
				Name:        "remove-signatures",
				Usage:       "Do not copy signatures from SOURCE-IMAGE",
				Destination: &opts.removeSignatures,
			},
			cli.StringFlag{
				Name:        "sign-by",
				Usage:       "Sign the image using a GPG key with the specified `FINGERPRINT`",
				Destination: &opts.signByFingerprint,
			},
			cli.GenericFlag{
				Name:  "format, f",
				Usage: "`MANIFEST TYPE` (oci, v2s1, or v2s2) to use when saving image to directory using the 'dir:' transport (default is manifest type of source)",
				Value: newOptionalStringValue(&opts.format),
			},
			cli.BoolFlag{
				Name:        "try-torrent",
				Usage:       "attempt to pull layers using BitTorrent",
				Destination: &opts.dockerTryTorrent,
			},
			cli.StringSliceFlag{
				Name:  "torrent-trackers",
				Usage: "additional trackers for BitTorrent",
				Value: &opts.dockerTorrentTrackers,
			},
		}, sharedFlags...), srcFlags...), destFlags...),
	}
}

func (opts *copyOptions) run(args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return errorShouldDisplayUsage{errors.New("Exactly two arguments expected")}
	}

	policyContext, err := opts.global.getPolicyContext()
	if err != nil {
		return fmt.Errorf("Error loading trust policy: %v", err)
	}
	defer policyContext.Destroy()

	srcRef, err := alltransports.ParseImageName(args[0])
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", args[0], err)
	}
	destRef, err := alltransports.ParseImageName(args[1])
	if err != nil {
		return fmt.Errorf("Invalid destination name %s: %v", args[1], err)
	}

	sourceCtx, err := opts.srcImage.newSystemContext()
	if err != nil {
		return err
	}
	destinationCtx, err := opts.destImage.newSystemContext()
	if err != nil {
		return err
	}
	if opts.dockerTryTorrent {
		sourceCtx.DockerTryTorrent = true
	}
	if opts.dockerTorrentTrackers != nil {
		sourceCtx.DockerTorrentTrackers = opts.dockerTorrentTrackers
	}

	var manifestType string
	if opts.format.present {
		switch opts.format.value {
		case "oci":
			manifestType = imgspecv1.MediaTypeImageManifest
		case "v2s1":
			manifestType = manifest.DockerV2Schema1SignedMediaType
		case "v2s2":
			manifestType = manifest.DockerV2Schema2MediaType
		default:
			return fmt.Errorf("unknown format %q. Choose one of the supported formats: 'oci', 'v2s1', or 'v2s2'", opts.format.value)
		}
	}

	for _, image := range opts.additionalTags {
		ref, err := reference.ParseNormalizedNamed(image)
		if err != nil {
			return fmt.Errorf("error parsing additional-tag '%s': %v", image, err)
		}
		namedTagged, isNamedTagged := ref.(reference.NamedTagged)
		if !isNamedTagged {
			return fmt.Errorf("additional-tag '%s' must be a tagged reference", image)
		}
		destinationCtx.DockerArchiveAdditionalTags = append(destinationCtx.DockerArchiveAdditionalTags, namedTagged)
	}

	ctx, cancel := opts.global.commandTimeoutContext()
	defer cancel()

	_, err = copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{
		RemoveSignatures:      opts.removeSignatures,
		SignBy:                opts.signByFingerprint,
		ReportWriter:          stdout,
		SourceCtx:             sourceCtx,
		DestinationCtx:        destinationCtx,
		ForceManifestMIMEType: manifestType,
	})
	return err
}
