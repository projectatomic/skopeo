package main

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/containers/image/signature"
	"github.com/containers/image/transports"
	"github.com/urfave/cli"
)

func standaloneSign(context *cli.Context) error {
	outputFile := context.String("output")
	if len(context.Args()) != 3 || outputFile == "" {
		return errors.New("Usage: skopeo standalone-sign manifest docker-reference key-fingerprint -o signature")
	}
	manifestPath := context.Args()[0]
	dockerReference := context.Args()[1]
	fingerprint := context.Args()[2]

	manifest, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("Error reading %s: %v", manifestPath, err)
	}

	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return fmt.Errorf("Error initializing GPG: %v", err)
	}
	signature, err := signature.SignDockerManifest(manifest, dockerReference, mech, fingerprint)
	if err != nil {
		return fmt.Errorf("Error creating signature: %v", err)
	}

	if err := ioutil.WriteFile(outputFile, signature, 0644); err != nil {
		return fmt.Errorf("Error writing signature to %s: %v", outputFile, err)
	}
	return nil
}

var standaloneSignCmd = cli.Command{
	Name:      "standalone-sign",
	Usage:     "Create a signature using local files",
	ArgsUsage: "MANIFEST DOCKER-REFERENCE KEY-FINGERPRINT",
	Action:    standaloneSign,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "output the signature to `SIGNATURE`",
		},
	},
}

func standaloneVerify(context *cli.Context) error {
	if len(context.Args()) != 4 {
		return errors.New("Usage: skopeo standalone-verify manifest docker-reference key-fingerprint signature")
	}
	manifestPath := context.Args()[0]
	expectedDockerReference := context.Args()[1]
	expectedFingerprint := context.Args()[2]
	signaturePath := context.Args()[3]

	unverifiedManifest, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("Error reading manifest from %s: %v", manifestPath, err)
	}
	unverifiedSignature, err := ioutil.ReadFile(signaturePath)
	if err != nil {
		return fmt.Errorf("Error reading signature from %s: %v", signaturePath, err)
	}

	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return fmt.Errorf("Error initializing GPG: %v", err)
	}
	sig, err := signature.VerifyDockerManifestSignature(unverifiedSignature, unverifiedManifest, expectedDockerReference, mech, expectedFingerprint)
	if err != nil {
		return fmt.Errorf("Error verifying signature: %v", err)
	}

	fmt.Fprintf(context.App.Writer, "Signature verified, digest %s\n", sig.DockerManifestDigest)
	return nil
}

var standaloneVerifyCmd = cli.Command{
	Name:      "standalone-verify",
	Usage:     "Verify a signature using local files",
	ArgsUsage: "MANIFEST DOCKER-REFERENCE KEY-FINGERPRINT SIGNATURE",
	Action:    standaloneVerify,
}

func addSignature(context *cli.Context) error {
	if len(context.Args()) != 2 {
		return errors.New("Usage: skopeo add-signature reference key-fingerprint")
	}
	ref := context.Args()[0]
	fingerprint := context.Args()[1]

	imageRef, err := transports.ParseImageName(ref)
	if err != nil {
		return fmt.Errorf("Error parsing %s: %v", ref, err)
	}
	name := transports.ImageName(imageRef)

	reference := context.String("reference")
	if reference == "" {
		signReference := imageRef.DockerReference()
		if signReference == nil {
			return fmt.Errorf("No reference associated with image %s: %v", ref, err)
		}
		reference = signReference.String()
	} else {
		signRef, err := imageRef.Transport().ParseReference(reference)
		if err != nil {
			return fmt.Errorf("Error parsing %s: %v", reference, err)
		}
		signReference := signRef.DockerReference()
		if signReference != nil {
			reference = signReference.String()
		}
	}

	systemContext := contextFromGlobalOptions(context)
	src, err := imageRef.NewImageSource(systemContext, nil)
	if err != nil {
		return fmt.Errorf("Error reading %s: %v", name, err)
	}
	defer src.Close()

	manifest, _, err := src.GetManifest()
	if err != nil {
		return fmt.Errorf("Error reading manifest for %s: %v", name, err)
	}

	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return fmt.Errorf("Error initializing GPG: %v", err)
	}

	signature, err := signature.SignDockerManifest(manifest, reference, mech, fingerprint)
	if err != nil {
		return fmt.Errorf("Error creating signature: %v", err)
	}

	dest, err := imageRef.NewImageDestination(systemContext)
	if err != nil {
		return fmt.Errorf("Error writing to %s: %v", name, err)
	}
	defer dest.Close()

	signatures, err := src.GetSignatures()
	if err != nil {
		return fmt.Errorf("Error reading signatures from %s: %v", name, err)
	}

	signatures = append(signatures, signature)

	err = dest.PutSignatures(signatures)
	if err != nil {
		return fmt.Errorf("Error saving signatures for %s: %v", name, err)
	}

	err = dest.Commit()
	if err != nil {
		return fmt.Errorf("Error saving updates to %s: %v", name, err)
	}

	return nil
}

var addSignatureCmd = cli.Command{
	Name:      "add-signature",
	Usage:     "Add a signature to an image",
	ArgsUsage: "IMAGE-REFERENCE KEY-FINGERPRINT",
	Action:    addSignature,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "reference, r",
			Usage: "sign a specified reference instead the image's default",
		},
	},
}

func verifySignature(context *cli.Context) error {
	var sig *signature.Signature

	if len(context.Args()) != 2 {
		return errors.New("Usage: skopeo verify-signature reference key-fingerprint")
	}
	ref := context.Args()[0]
	expectedFingerprint := context.Args()[1]

	imageRef, err := transports.ParseImageName(ref)
	if err != nil {
		return fmt.Errorf("Error parsing %s: %v", ref, err)
	}
	name := transports.ImageName(imageRef)

	reference := context.String("reference")
	if reference == "" {
		verifyReference := imageRef.DockerReference()
		if verifyReference == nil {
			return fmt.Errorf("No reference associated with image %s: %v", ref, err)
		}
		reference = verifyReference.String()
	} else {
		verifyRef, err := imageRef.Transport().ParseReference(reference)
		if err != nil {
			return fmt.Errorf("Error parsing %s: %v", reference, err)
		}
		verifyReference := verifyRef.DockerReference()
		if verifyReference != nil {
			reference = verifyReference.String()
		}
	}

	systemContext := contextFromGlobalOptions(context)
	src, err := imageRef.NewImageSource(systemContext, nil)
	if err != nil {
		return fmt.Errorf("Error reading %s: %v", name, err)
	}
	defer src.Close()

	unverifiedManifest, _, err := src.GetManifest()
	if err != nil {
		return fmt.Errorf("Error reading manifest for %s: %v", name, err)
	}

	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return fmt.Errorf("Error initializing GPG: %v", err)
	}

	signatures, err := src.GetSignatures()
	if err != nil {
		return fmt.Errorf("Error reading signatures from %s: %v", name, err)
	}

	if len(signatures) == 0 {
		return fmt.Errorf("No signatures found: %v", err)
	}

	// TODO: is there a better way to zero in on which signature we want to check, instead of checking them all for a match with our criteria?
	for _, unverifiedSignature := range signatures {
		sig, err = signature.VerifyDockerManifestSignature(unverifiedSignature, unverifiedManifest, reference, mech, expectedFingerprint)
		if err == nil {
			fmt.Fprintf(context.App.Writer, "Signature verified, digest %s\n", sig.DockerManifestDigest)
			return nil
		}
	}

	return fmt.Errorf("Error verifying signature: %v", err)
}

var verifySignatureCmd = cli.Command{
	Name:      "verify-signature",
	Usage:     "Verify a signature on an image",
	ArgsUsage: "IMAGE-REFERENCE KEY-FINGERPRINT",
	Action:    verifySignature,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "reference, r",
			Usage: "verify a specified reference instead of one associated with the image name",
		},
	},
}
