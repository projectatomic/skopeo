package directory

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/image/types"
)

type dirImageDestination struct {
	dir string
}

// NewDirImageDestination returns an ImageDestination for writing to an existing directory.
func NewDirImageDestination(dir string) types.ImageDestination {
	return &dirImageDestination{dir}
}

func (d *dirImageDestination) CanonicalDockerReference() (string, error) {
	return "", fmt.Errorf("Can not determine canonical Docker reference for a local directory")
}

func (d *dirImageDestination) PutManifest(manifest []byte) error {
	return ioutil.WriteFile(manifestPath(d.dir), manifest, 0644)
}

func (d *dirImageDestination) PutBlob(digest string, stream io.Reader) error {
	layerFile, err := os.Create(layerPath(d.dir, digest))
	if err != nil {
		return err
	}
	defer layerFile.Close()
	if _, err := io.Copy(layerFile, stream); err != nil {
		return err
	}
	if err := layerFile.Sync(); err != nil {
		return err
	}
	return nil
}

func (d *dirImageDestination) PutSignatures(signatures [][]byte) error {
	for i, sig := range signatures {
		if err := ioutil.WriteFile(signaturePath(d.dir, i), sig, 0644); err != nil {
			return err
		}
	}
	return nil
}
