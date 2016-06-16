package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/projectatomic/skopeo/docker"
	"github.com/projectatomic/skopeo/docker/utils"
	"github.com/urfave/cli"
)

// inspectOutput is the output format of (skopeo inspect), primarily so that we can format it with a simple json.MarshalIndent.
type inspectOutput struct {
	Name          string `json:",omitempty"`
	Tag           string
	Digest        string
	RepoTags      []string
	Created       time.Time
	DockerVersion string
	Labels        map[string]string
	Architecture  string
	Os            string
	Layers        []string
}

var inspectCmd = cli.Command{
	Name:  "inspect",
	Usage: "inspect images on a registry",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "raw",
			Usage: "output raw manifest",
		},
	},
	Action: func(c *cli.Context) {
		img, err := parseImage(c)
		if err != nil {
			logrus.Fatal(err)
		}
		rawManifest, err := img.Manifest()
		if err != nil {
			logrus.Fatal(err)
		}
		if c.Bool("raw") {
			fmt.Println(string(rawManifest))
			return
		}
		imgInspect, err := img.Inspect()
		if err != nil {
			logrus.Fatal(err)
		}
		outputData := inspectOutput{
			Name: "", // Possibly overridden for a docker.Image.
			Tag:  imgInspect.Tag,
			// Digest is set below.
			RepoTags:      []string{}, // Possibly overriden for a docker.Image.
			Created:       imgInspect.Created,
			DockerVersion: imgInspect.DockerVersion,
			Labels:        imgInspect.Labels,
			Architecture:  imgInspect.Architecture,
			Os:            imgInspect.Os,
			Layers:        imgInspect.Layers,
		}
		outputData.Digest, err = utils.ManifestDigest(rawManifest)
		if err != nil {
			logrus.Fatalf("Error computing manifest digest: %s", err.Error())
		}
		if dockerImg, ok := img.(*docker.Image); ok {
			outputData.Name = dockerImg.SourceRefFullName()
			outputData.RepoTags, err = dockerImg.GetRepositoryTags()
			if err != nil {
				logrus.Fatalf("Error determining repository tags: %s", err.Error())
			}
		}
		out, err := json.MarshalIndent(outputData, "", "    ")
		if err != nil {
			logrus.Fatal(err)
		}
		fmt.Println(string(out))
	},
}
