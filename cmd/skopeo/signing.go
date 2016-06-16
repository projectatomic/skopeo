package main

import (
	"fmt"
	"io/ioutil"

	"github.com/Sirupsen/logrus"
	"github.com/projectatomic/skopeo/signature"
	"github.com/urfave/cli"
)

func standaloneSign(context *cli.Context) {
	outputFile := context.String("output")
	if len(context.Args()) != 3 || outputFile == "" {
		logrus.Fatal("Usage: skopeo standalone-sign manifest docker-reference key-fingerprint -o signature")
	}
	manifestPath := context.Args()[0]
	dockerReference := context.Args()[1]
	fingerprint := context.Args()[2]

	manifest, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		logrus.Fatalf("Error reading %s: %s", manifestPath, err.Error())
	}

	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		logrus.Fatalf("Error initializing GPG: %s", err.Error())
	}
	signature, err := signature.SignDockerManifest(manifest, dockerReference, mech, fingerprint)
	if err != nil {
		logrus.Fatalf("Error creating signature: %s", err.Error())
	}

	if err := ioutil.WriteFile(outputFile, signature, 0644); err != nil {
		logrus.Fatalf("Error writing signature to %s: %s", outputFile, err.Error())
	}
}

var standaloneSignCmd = cli.Command{
	Name:   "standalone-sign",
	Usage:  "Create a signature using local files",
	Action: standaloneSign,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "output signature file name",
		},
	},
}

func standaloneVerify(context *cli.Context) {
	if len(context.Args()) != 4 {
		logrus.Fatal("Usage: skopeo standalone-verify manifest docker-reference key-fingerprint signature")
	}
	manifestPath := context.Args()[0]
	expectedDockerReference := context.Args()[1]
	expectedFingerprint := context.Args()[2]
	signaturePath := context.Args()[3]

	unverifiedManifest, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		logrus.Fatalf("Error reading manifest from %s: %s", signaturePath, err.Error())
	}
	unverifiedSignature, err := ioutil.ReadFile(signaturePath)
	if err != nil {
		logrus.Fatalf("Error reading signature from %s: %s", signaturePath, err.Error())
	}

	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		logrus.Fatalf("Error initializing GPG: %s", err.Error())
	}
	sig, err := signature.VerifyDockerManifestSignature(unverifiedSignature, unverifiedManifest, expectedDockerReference, mech, expectedFingerprint)
	if err != nil {
		logrus.Fatalf("Error verifying signature: %s", err.Error())
	}

	fmt.Printf("Signature verified, digest %s\n", sig.DockerManifestDigest)
}

var standaloneVerifyCmd = cli.Command{
	Name:   "standalone-verify",
	Usage:  "Verify a signature using local files",
	Action: standaloneVerify,
}
