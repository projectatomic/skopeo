package main

import (
	"errors"
	"fmt"

	"github.com/projectatomic/skopeo/image"
	"github.com/projectatomic/skopeo/signature"
	"github.com/urfave/cli"
)

func copyHandler(context *cli.Context) error {
	if len(context.Args()) != 2 {
		return errors.New("Usage: copy source destination")
	}

	rawSource, err := parseImageSource(context, context.Args()[0])
	if err != nil {
		return fmt.Errorf("Error initializing %s: %v", context.Args()[0], err)
	}
	src := image.FromSource(rawSource)

	dest, err := parseImageDestination(context, context.Args()[1])
	if err != nil {
		return fmt.Errorf("Error initializing %s: %v", context.Args()[1], err)
	}
	signBy := context.String("sign-by")

	manifest, err := src.Manifest()
	if err != nil {
		return fmt.Errorf("Error reading manifest: %v", err)
	}

	layers, err := src.LayerDigests()
	if err != nil {
		return fmt.Errorf("Error parsing manifest: %v", err)
	}
	for _, layer := range layers {
		// TODO(mitr): do not ignore the size param returned here
		stream, _, err := rawSource.GetBlob(layer)
		if err != nil {
			return fmt.Errorf("Error reading layer %s: %v", layer, err)
		}
		defer stream.Close()
		if err := dest.PutBlob(layer, stream); err != nil {
			return fmt.Errorf("Error writing layer: %v", err)
		}
	}

	sigs, err := src.Signatures()
	if err != nil {
		return fmt.Errorf("Error reading signatures: %v", err)
	}

	if signBy != "" {
		mech, err := signature.NewGPGSigningMechanism()
		if err != nil {
			return fmt.Errorf("Error initializing GPG: %v", err)
		}
		dockerReference, err := dest.CanonicalDockerReference()
		if err != nil {
			return fmt.Errorf("Error determining canonical Docker reference: %v", err)
		}

		newSig, err := signature.SignDockerManifest(manifest, dockerReference, mech, signBy)
		if err != nil {
			return fmt.Errorf("Error creating signature: %v", err)
		}
		sigs = append(sigs, newSig)
	}

	if err := dest.PutSignatures(sigs); err != nil {
		return fmt.Errorf("Error writing signatures: %v", err)
	}

	// FIXME: We need to call PutManifest after PutBlob and PutSignatures. This seems ugly; move to a "set properties" + "commit" model?
	if err := dest.PutManifest(manifest); err != nil {
		return fmt.Errorf("Error writing manifest: %v", err)
	}
	return nil
}

var copyCmd = cli.Command{
	Name:   "copy",
	Action: copyHandler,
	// FIXME: Do we need to namespace the GPG aspect?
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "sign-by",
			Usage: "sign the image using a GPG key with the specified fingerprint",
		},
	},
}
