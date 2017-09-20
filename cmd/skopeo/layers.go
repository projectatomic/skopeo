package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/image/directory"
	"github.com/containers/image/image"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var layersCmd = cli.Command{
	Name:      "layers",
	Usage:     "Get layers of IMAGE-NAME",
	ArgsUsage: "IMAGE-NAME [LAYER...]",
	Hidden:    true,
	Action: func(c *cli.Context) (retErr error) {
		fmt.Fprintln(os.Stderr, `DEPRECATED: skopeo layers is deprecated in favor of skopeo copy`)
		if c.NArg() == 0 {
			return errors.New("Usage: layers imageReference [layer...]")
		}
		rawSource, err := parseImageSource(c, c.Args()[0])
		if err != nil {
			return err
		}
		src, err := image.FromSource(rawSource)
		if err != nil {
			if closeErr := rawSource.Close(); closeErr != nil {
				return errors.Wrapf(err, " (close error: %v)", closeErr)
			}

			return err
		}
		defer func() {
			if err := src.Close(); err != nil {
				retErr = errors.Wrapf(retErr, " (close error: %v)", err)
			}
		}()

		var blobDigests []digest.Digest
		for _, dString := range c.Args().Tail() {
			if !strings.HasPrefix(dString, "sha256:") {
				dString = "sha256:" + dString
			}
			d, err := digest.Parse(dString)
			if err != nil {
				return err
			}
			blobDigests = append(blobDigests, d)
		}

		if len(blobDigests) == 0 {
			layers := src.LayerInfos()
			seenLayers := map[digest.Digest]struct{}{}
			for _, info := range layers {
				if _, ok := seenLayers[info.Digest]; !ok {
					blobDigests = append(blobDigests, info.Digest)
					seenLayers[info.Digest] = struct{}{}
				}
			}
			configInfo := src.ConfigInfo()
			if configInfo.Digest != "" {
				blobDigests = append(blobDigests, configInfo.Digest)
			}
		}

		tmpDir, err := ioutil.TempDir(".", "layers-")
		if err != nil {
			return err
		}
		tmpDirRef, err := directory.NewReference(tmpDir)
		if err != nil {
			return err
		}
		dest, err := tmpDirRef.NewImageDestination(nil)
		if err != nil {
			return err
		}

		defer func() {
			if err := dest.Close(); err != nil {
				retErr = errors.Wrapf(retErr, " (close error: %v)", err)
			}
		}()

		for _, digest := range blobDigests {
			r, blobSize, err := rawSource.GetBlob(types.BlobInfo{Digest: digest, Size: -1})
			if err != nil {
				return err
			}
			if _, err := dest.PutBlob(r, types.BlobInfo{Digest: digest, Size: blobSize}); err != nil {
				if closeErr := r.Close(); closeErr != nil {
					return errors.Wrapf(err, " (close error: %v)", closeErr)
				}
				return err
			}
		}

		manifest, _, err := src.Manifest()
		if err != nil {
			return err
		}
		if err := dest.PutManifest(manifest); err != nil {
			return err
		}

		return dest.Commit()
	},
}
