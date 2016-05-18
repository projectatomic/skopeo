package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/projectatomic/skopeo/docker/utils"
)

// inspectOutput is the output format of (skopeo inspect), primarily so that we can format it with a simple json.MarshalIndent.
type inspectOutput struct {
	Name          string
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
		rawManifest, _, err := img.Manifest()
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
		manifestDigest, err := utils.ManifestDigest(rawManifest)
		if err != nil {
			logrus.Fatalf("Error computing manifest digest: %s", err.Error())
		}
		repoTags, err := img.GetRepositoryTags()
		if err != nil {
			logrus.Fatalf("Error determining repository tags: %s", err.Error())
		}
		outputData := inspectOutput{
			Name:          imgInspect.Name,
			Tag:           imgInspect.Tag,
			Digest:        manifestDigest,
			RepoTags:      repoTags,
			Created:       imgInspect.Created,
			DockerVersion: imgInspect.DockerVersion,
			Labels:        imgInspect.Labels,
			Architecture:  imgInspect.Architecture,
			Os:            imgInspect.Os,
			Layers:        imgInspect.Layers,
		}
		out, err := json.MarshalIndent(outputData, "", "    ")
		if err != nil {
			logrus.Fatal(err)
		}
		fmt.Println(string(out))
	},
}
