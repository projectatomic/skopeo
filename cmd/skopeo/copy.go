package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/spf13/cobra"

	encconfig "github.com/containers/ocicrypt/config"
	enchelpers "github.com/containers/ocicrypt/helpers"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type copyOptions struct {
	global            *globalOptions
	srcImage          *imageOptions
	destImage         *imageDestOptions
	retryOpts         *retry.RetryOptions
	additionalTags    []string       // For docker-archive: destinations, in addition to the name:tag specified as destination, also add these
	removeSignatures  bool           // Do not copy signatures from the source image
	signByFingerprint string         // Sign the image using a GPG key with the specified fingerprint
	format            optionalString // Force conversion of the image to a specified format
	quiet             bool           // Suppress output information when copying images
	all               bool           // Copy all of the images if the source is a list
	encryptLayer      []int          // The list of layers to encrypt
	encryptionKeys    []string       // Keys needed to encrypt the image
	decryptionKeys    []string       // Keys needed to decrypt the image
}

func copyCmd(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	srcFlags, srcOpts := imageFlags(global, sharedOpts, "src-", "screds")
	destFlags, destOpts := imageDestFlags(global, sharedOpts, "dest-", "dcreds")
	retryFlags, retryOpts := retryFlags()
	opts := copyOptions{global: global,
		srcImage:  srcOpts,
		destImage: destOpts,
		retryOpts: retryOpts,
	}
	cmd := &cobra.Command{
		Use:   "copy [command options] SOURCE-IMAGE DESTINATION-IMAGE",
		Short: "Copy an IMAGE-NAME from one location to another",
		Long: fmt.Sprintf(`Container "IMAGE-NAME" uses a "transport":"details" format.

Supported transports:
%s

See skopeo(1) section "IMAGE NAMES" for the expected format
`, strings.Join(transports.ListNames(), ", ")),
		RunE:    commandAction(opts.run),
		Example: `skopeo copy docker://quay.io/skopeo/stable:latest docker://registry.example.com/skopeo:latest`,
	}
	adjustUsage(cmd)
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedFlags)
	flags.AddFlagSet(&srcFlags)
	flags.AddFlagSet(&destFlags)
	flags.AddFlagSet(&retryFlags)
	flags.StringSliceVar(&opts.additionalTags, "additional-tag", []string{}, "additional tags (supports docker-archive)")
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Suppress output information when copying images")
	flags.BoolVarP(&opts.all, "all", "a", false, "Copy all images if SOURCE-IMAGE is a list")
	flags.BoolVar(&opts.removeSignatures, "remove-signatures", false, "Do not copy signatures from SOURCE-IMAGE")
	flags.StringVar(&opts.signByFingerprint, "sign-by", "", "Sign the image using a GPG key with the specified `FINGERPRINT`")
	flags.VarP(newOptionalStringValue(&opts.format), "format", "f", `MANIFEST TYPE (oci, v2s1, or v2s2) to use when saving image to directory using the 'dir:' transport (default is manifest type of source)`)
	flags.StringSliceVar(&opts.encryptionKeys, "encryption-key", []string{}, "*Experimental* key with the encryption protocol to use needed to encrypt the image (e.g. jwe:/path/to/key.pem)")
	flags.IntSliceVar(&opts.encryptLayer, "encrypt-layer", []int{}, "*Experimental* the 0-indexed layer indices, with support for negative indexing (e.g. 0 is the first layer, -1 is the last layer)")
	flags.StringSliceVar(&opts.decryptionKeys, "decryption-key", []string{}, "*Experimental* key needed to decrypt the image")
	return cmd
}

func (opts *copyOptions) run(args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return errorShouldDisplayUsage{errors.New("Exactly two arguments expected")}
	}
	imageNames := args

	if err := reexecIfNecessaryForImages(imageNames...); err != nil {
		return err
	}

	policyContext, err := opts.global.getPolicyContext()
	if err != nil {
		return fmt.Errorf("Error loading trust policy: %v", err)
	}
	defer policyContext.Destroy()

	srcRef, err := alltransports.ParseImageName(imageNames[0])
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", imageNames[0], err)
	}
	destRef, err := alltransports.ParseImageName(imageNames[1])
	if err != nil {
		return fmt.Errorf("Invalid destination name %s: %v", imageNames[1], err)
	}

	sourceCtx, err := opts.srcImage.newSystemContext()
	if err != nil {
		return err
	}
	destinationCtx, err := opts.destImage.newSystemContext()
	if err != nil {
		return err
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

	if opts.quiet {
		stdout = nil
	}
	imageListSelection := copy.CopySystemImage
	if opts.all {
		imageListSelection = copy.CopyAllImages
	}

	if len(opts.encryptionKeys) > 0 && len(opts.decryptionKeys) > 0 {
		return fmt.Errorf("--encryption-key and --decryption-key cannot be specified together")
	}

	var encLayers *[]int
	var encConfig *encconfig.EncryptConfig
	var decConfig *encconfig.DecryptConfig

	if len(opts.encryptLayer) > 0 && len(opts.encryptionKeys) == 0 {
		return fmt.Errorf("--encrypt-layer can only be used with --encryption-key")
	}

	if len(opts.encryptionKeys) > 0 {
		// encryption
		p := opts.encryptLayer
		encLayers = &p
		encryptionKeys := opts.encryptionKeys
		ecc, err := enchelpers.CreateCryptoConfig(encryptionKeys, []string{})
		if err != nil {
			return fmt.Errorf("Invalid encryption keys: %v", err)
		}
		cc := encconfig.CombineCryptoConfigs([]encconfig.CryptoConfig{ecc})
		encConfig = cc.EncryptConfig
	}

	if len(opts.decryptionKeys) > 0 {
		// decryption
		decryptionKeys := opts.decryptionKeys
		dcc, err := enchelpers.CreateCryptoConfig([]string{}, decryptionKeys)
		if err != nil {
			return fmt.Errorf("Invalid decryption keys: %v", err)
		}
		cc := encconfig.CombineCryptoConfigs([]encconfig.CryptoConfig{dcc})
		decConfig = cc.DecryptConfig
	}

	return retry.RetryIfNecessary(ctx, func() error {
		_, err = copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{
			RemoveSignatures:      opts.removeSignatures,
			SignBy:                opts.signByFingerprint,
			ReportWriter:          stdout,
			SourceCtx:             sourceCtx,
			DestinationCtx:        destinationCtx,
			ForceManifestMIMEType: manifestType,
			ImageListSelection:    imageListSelection,
			OciDecryptConfig:      decConfig,
			OciEncryptLayers:      encLayers,
			OciEncryptConfig:      encConfig,
		})
		return err
	}, opts.retryOpts)
}
