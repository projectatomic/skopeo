package directory

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/image/types"
)

type dirImageSource struct {
	dir string
}

// NewDirImageSource returns an ImageSource reading from an existing directory.
func NewDirImageSource(dir string) types.ImageSource {
	return &dirImageSource{dir}
}

// IntendedDockerReference returns the full, unambiguous, Docker reference for this image, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
// May be "" if unknown.
func (s *dirImageSource) IntendedDockerReference() string {
	return ""
}

// it's up to the caller to determine the MIME type of the returned manifest's bytes
func (s *dirImageSource) GetManifest(_ []string) ([]byte, string, error) {
	m, err := ioutil.ReadFile(manifestPath(s.dir))
	if err != nil {
		return nil, "", err
	}
	return m, "", err
}

func (s *dirImageSource) GetBlob(digest string) (io.ReadCloser, int64, error) {
	r, err := os.Open(layerPath(s.dir, digest))
	if err != nil {
		return nil, 0, nil
	}
	fi, err := os.Stat(layerPath(s.dir, digest))
	if err != nil {
		return nil, 0, nil
	}
	return r, fi.Size(), nil
}

func (s *dirImageSource) GetSignatures() ([][]byte, error) {
	signatures := [][]byte{}
	for i := 0; ; i++ {
		signature, err := ioutil.ReadFile(signaturePath(s.dir, i))
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return nil, err
		}
		signatures = append(signatures, signature)
	}
	return signatures, nil
}

func (s *dirImageSource) Delete() error {
	return fmt.Errorf("directory#dirImageSource.Delete() not implmented")
}
