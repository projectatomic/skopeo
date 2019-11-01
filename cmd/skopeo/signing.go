package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/containers/image/v5/signature"
	"github.com/urfave/cli"
)

type standaloneSignOptions struct {
	output string // Output file path
}

func standaloneSignCmd() cli.Command {
	opts := standaloneSignOptions{}
	return cli.Command{
		Name:      "standalone-sign",
		Usage:     "Create a signature using local files",
		ArgsUsage: "MANIFEST DOCKER-REFERENCE KEY-FINGERPRINT",
		Action:    commandAction(opts.run),
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:        "output, o",
				Usage:       "output the signature to `SIGNATURE`",
				Destination: &opts.output,
			},
		},
	}
}

func (opts *standaloneSignOptions) run(args []string, stdout io.Writer) error {
	if len(args) != 3 || opts.output == "" {
		return errors.New("Usage: skopeo standalone-sign manifest docker-reference key-fingerprint -o signature")
	}
	manifestPath := args[0]
	dockerReference := args[1]
	fingerprint := args[2]

	manifest, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("Error reading %s: %v", manifestPath, err)
	}

	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return fmt.Errorf("Error initializing GPG: %v", err)
	}
	defer mech.Close()
	signature, err := signature.SignDockerManifest(manifest, dockerReference, mech, fingerprint)
	if err != nil {
		return fmt.Errorf("Error creating signature: %v", err)
	}

	if err := ioutil.WriteFile(opts.output, signature, 0644); err != nil {
		return fmt.Errorf("Error writing signature to %s: %v", opts.output, err)
	}
	return nil
}

type standaloneVerifyOptions struct {
}

func standaloneVerifyCmd() cli.Command {
	opts := standaloneVerifyOptions{}
	return cli.Command{
		Name:      "standalone-verify",
		Usage:     "Verify a signature using local files",
		ArgsUsage: "MANIFEST DOCKER-REFERENCE KEY-FINGERPRINT SIGNATURE",
		Action:    commandAction(opts.run),
	}
}

func (opts *standaloneVerifyOptions) run(args []string, stdout io.Writer) error {
	if len(args) != 4 {
		return errors.New("Usage: skopeo standalone-verify manifest docker-reference key-fingerprint signature")
	}
	manifestPath := args[0]
	expectedDockerReference := args[1]
	expectedFingerprint := args[2]
	signaturePath := args[3]

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
	defer mech.Close()
	sig, err := signature.VerifyDockerManifestSignature(unverifiedSignature, unverifiedManifest, expectedDockerReference, mech, expectedFingerprint)
	if err != nil {
		return fmt.Errorf("Error verifying signature: %v", err)
	}

	fmt.Fprintf(stdout, "Signature verified, digest %s\n", sig.DockerManifestDigest)
	return nil
}

// WARNING: Do not use the contents of this for ANY security decisions,
// and be VERY CAREFUL about showing this information to humans in any way which suggest that these values “are probably” reliable.
// There is NO REASON to expect the values to be correct, or not intentionally misleading
// (including things like “✅ Verified by $authority”)
//
// The subcommand is undocumented, and it may be renamed or entirely disappear in the future.
type untrustedSignatureDumpOptions struct {
}

func untrustedSignatureDumpCmd() cli.Command {
	opts := untrustedSignatureDumpOptions{}
	return cli.Command{
		Name:      "untrusted-signature-dump-without-verification",
		Usage:     "Dump contents of a signature WITHOUT VERIFYING IT",
		ArgsUsage: "SIGNATURE",
		Hidden:    true,
		Action:    commandAction(opts.run),
	}
}

func (opts *untrustedSignatureDumpOptions) run(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("Usage: skopeo untrusted-signature-dump-without-verification signature")
	}
	untrustedSignaturePath := args[0]

	untrustedSignature, err := ioutil.ReadFile(untrustedSignaturePath)
	if err != nil {
		return fmt.Errorf("Error reading untrusted signature from %s: %v", untrustedSignaturePath, err)
	}

	untrustedInfo, err := signature.GetUntrustedSignatureInformationWithoutVerifying(untrustedSignature)
	if err != nil {
		return fmt.Errorf("Error decoding untrusted signature: %v", err)
	}
	untrustedOut, err := json.MarshalIndent(untrustedInfo, "", "    ")
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, string(untrustedOut))
	return nil
}
