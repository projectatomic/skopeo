package main

import (
	"io/ioutil"
	"strings"

	"github.com/containers/image/directory"
	"github.com/containers/image/image"
	"github.com/urfave/cli"
)

// TODO(runcom): document args and usage
var layersCmd = cli.Command{
	Name:  "layers",
	Usage: "get images layers",
	Action: func(c *cli.Context) error {
		rawSource, err := parseImageSource(c, c.Args()[0])
		if err != nil {
			return err
		}
		src := image.FromSource(rawSource)
		layers := c.Args().Tail()
		if len(layers) == 0 {
			ls, err := src.BlobDigests()
			if err != nil {
				return err
			}
			layers = ls
		}
		tmpDir, err := ioutil.TempDir(".", "layers-")
		if err != nil {
			return err
		}
		dest := directory.NewDirImageDestination(tmpDir)
		manifest, _, err := src.Manifest()
		if err != nil {
			return err
		}
		if err := dest.PutManifest(manifest); err != nil {
			return err
		}
		for _, l := range layers {
			if !strings.HasPrefix(l, "sha256:") {
				l = "sha256:" + l
			}
			r, _, err := rawSource.GetBlob(l)
			if err != nil {
				return err
			}
			if err := dest.PutBlob(l, r); err != nil {
				r.Close()
				return err
			}
			r.Close()
		}
		return nil
	},
}
