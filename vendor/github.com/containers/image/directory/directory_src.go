package directory

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
)

type dirImageSource struct {
	ref dirReference
}

// newImageSource returns an ImageSource reading from an existing directory.
// The caller must call .Close() on the returned ImageSource.
func newImageSource(ref dirReference) types.ImageSource {
	return &dirImageSource{ref}
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *dirImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *dirImageSource) Close() {
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
func (s *dirImageSource) GetManifest() ([]byte, string, error) {
	m, err := ioutil.ReadFile(s.ref.manifestPath())
	if err != nil {
		return nil, "", err
	}
	return m, manifest.GuessMIMEType(m), err
}

func (s *dirImageSource) GetTargetManifest(digest string) ([]byte, string, error) {
	return nil, "", fmt.Errorf(`Getting target manifest not supported by "dir:"`)
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
func (s *dirImageSource) GetBlob(digest string) (io.ReadCloser, int64, error) {
	r, err := os.Open(s.ref.layerPath(digest))
	if err != nil {
		return nil, 0, nil
	}
	fi, err := r.Stat()
	if err != nil {
		return nil, 0, nil
	}
	return r, fi.Size(), nil
}

func (s *dirImageSource) GetSignatures() ([][]byte, error) {
	signatures := [][]byte{}
	for i := 0; ; i++ {
		signature, err := ioutil.ReadFile(s.ref.signaturePath(i))
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
