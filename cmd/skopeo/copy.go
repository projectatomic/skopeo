package main

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/projectatomic/skopeo/signature"
)

// FIXME: Also handle schema2, and put this elsewhere:
// docker.go contains similar code, more sophisticated
// (at the very least the deduplication should be reused from there).
func manifestLayers(manifest []byte) ([]string, error) {
	man := manifestSchema1{}
	if err := json.Unmarshal(manifest, &man); err != nil {
		return nil, err
	}

	layers := []string{}
	for _, layer := range man.FSLayers {
		layers = append(layers, layer.BlobSum)
	}
	return layers, nil
}

// FIXME: this is a copy from docker_image.go and does not belong here.
type manifestSchema1 struct {
	Name     string
	Tag      string
	FSLayers []struct {
		BlobSum string `json:"blobSum"`
	} `json:"fsLayers"`
	History []struct {
		V1Compatibility string `json:"v1Compatibility"`
	} `json:"history"`
	// TODO(runcom) verify the downloaded manifest
	//Signature []byte `json:"signature"`
}

func copyHandler(context *cli.Context) {
	if len(context.Args()) != 2 {
		logrus.Fatal("Usage: copy source destination")
	}

	src, err := parseImageSource(context, context.Args()[0])
	if err != nil {
		logrus.Fatalf("Error initializing %s: %s", context.Args()[0], err.Error())
	}

	dest, err := parseImageDestination(context, context.Args()[1])
	if err != nil {
		logrus.Fatalf("Error initializing %s: %s", context.Args()[1], err.Error())
	}
	signBy := context.String("sign-by")

	manifest, _, err := src.GetManifest()
	if err != nil {
		logrus.Fatalf("Error reading manifest: %s", err.Error())
	}

	layers, err := manifestLayers(manifest)
	if err != nil {
		logrus.Fatalf("Error parsing manifest: %s", err.Error())
	}
	for _, layer := range layers {
		stream, err := src.GetLayer(layer)
		if err != nil {
			logrus.Fatalf("Error reading layer %s: %s", layer, err.Error())
		}
		defer stream.Close()
		if err := dest.PutLayer(layer, stream); err != nil {
			logrus.Fatalf("Error writing layer: %s", err.Error())
		}
	}

	sigs, err := src.GetSignatures()
	if err != nil {
		logrus.Fatalf("Error reading signatures: %s", err.Error())
	}

	if signBy != "" {
		mech, err := signature.NewGPGSigningMechanism()
		if err != nil {
			logrus.Fatalf("Error initializing GPG: %s", err.Error())
		}
		dockerReference, err := dest.CanonicalDockerReference()
		if err != nil {
			logrus.Fatalf("Error determining canonical Docker reference: %s", err.Error())
		}

		newSig, err := signature.SignDockerManifest(manifest, dockerReference, mech, signBy)
		if err != nil {
			logrus.Fatalf("Error creating signature: %s", err.Error())
		}
		sigs = append(sigs, newSig)
	}

	if err := dest.PutSignatures(sigs); err != nil {
		logrus.Fatalf("Error writing signatures: %s", err.Error())
	}

	// FIXME: We need to call PutManifest after PutLayer and PutSignatures. This seems ugly; move to a "set properties" + "commit" model?
	if err := dest.PutManifest(manifest); err != nil {
		logrus.Fatalf("Error writing manifest: %s", err.Error())
	}
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
