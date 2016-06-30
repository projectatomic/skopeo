package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/containers/image/docker"
	"github.com/containers/image/manifest"
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
	Description: "the image is specified in this form: \"{prefix}{image}\", " +
		"supported prefixes are:\n" +
		"     * docker:// — docker image reference\n" +
		"     * atomic:   — atomic registry reference\n" +
		"     * dir:      — local directories (for debugging)\n" +
		"   e.g. docker://fedora",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "raw",
			Usage: "output raw manifest",
		},
	},
	Action: func(c *cli.Context) error {
		img, err := parseImage(c)
		if err != nil {
			return err
		}
		rawManifest, _, err := img.Manifest()
		if err != nil {
			return err
		}
		if c.Bool("raw") {
			fmt.Fprintln(c.App.Writer, string(rawManifest))
			return nil
		}
		imgInspect, err := img.Inspect()
		if err != nil {
			return err
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
		outputData.Digest, err = manifest.Digest(rawManifest)
		if err != nil {
			return fmt.Errorf("Error computing manifest digest: %v", err)
		}
		if dockerImg, ok := img.(*docker.Image); ok {
			outputData.Name = dockerImg.SourceRefFullName()
			outputData.RepoTags, err = dockerImg.GetRepositoryTags()
			if err != nil {
				return fmt.Errorf("Error determining repository tags: %v", err)
			}
		}
		out, err := json.MarshalIndent(outputData, "", "    ")
		if err != nil {
			return err
		}
		fmt.Fprintln(c.App.Writer, string(out))
		return nil
	},
}
