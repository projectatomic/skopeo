package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/containers/image/docker"
	"github.com/containers/image/manifest"
	"github.com/docker/docker/reference"
	"github.com/urfave/cli"
)

// inspectOutput is the output format of (skopeo inspect), primarily so that we can format it with a simple json.MarshalIndent.
type inspectOutput struct {
	Name          string `json:",omitempty"`
	Tag           string `json:",omitempty"`
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
	Name:      "inspect",
	Usage:     "Inspect image IMAGE-NAME",
	ArgsUsage: "IMAGE-NAME",
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
		defer img.Close()

		rawManifest, _, err := img.Manifest()
		if err != nil {
			return err
		}
		if c.Bool("raw") {
			_, err := c.App.Writer.Write(rawManifest)
			if err != nil {
				return fmt.Errorf("Error writing manifest to standard output: %v", err)
			}
			return nil
		}
		imgInspect, err := img.Inspect()
		if err != nil {
			return err
		}
		var tag string
		if tagged, ok := img.Reference().DockerReference().(reference.NamedTagged); ok {
			tag = tagged.Tag()
		}
		outputData := inspectOutput{
			Name: "", // Possibly overridden for a docker.Image.
			Tag:  tag,
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
