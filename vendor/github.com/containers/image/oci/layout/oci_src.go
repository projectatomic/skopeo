package layout

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type ociImageSource struct {
	ref ociReference
}

// newImageSource returns an ImageSource for reading from an existing directory.
func newImageSource(ref ociReference) types.ImageSource {
	return &ociImageSource{ref: ref}
}

// Reference returns the reference used to set up this source.
func (s *ociImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *ociImageSource) Close() {
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
func (s *ociImageSource) GetManifest() ([]byte, string, error) {
	descriptorPath := s.ref.descriptorPath(s.ref.tag)
	data, err := ioutil.ReadFile(descriptorPath)
	if err != nil {
		return nil, "", err
	}

	desc := imgspecv1.Descriptor{}
	err = json.Unmarshal(data, &desc)
	if err != nil {
		return nil, "", err
	}

	manifestPath, err := s.ref.blobPath(desc.Digest)
	if err != nil {
		return nil, "", err
	}
	m, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, "", err
	}

	return m, manifest.GuessMIMEType(m), nil
}

func (s *ociImageSource) GetTargetManifest(digest string) ([]byte, string, error) {
	manifestPath, err := s.ref.blobPath(digest)
	if err != nil {
		return nil, "", err
	}

	m, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, "", err
	}

	return m, manifest.GuessMIMEType(m), nil
}

// GetBlob returns a stream for the specified blob, and the blob's size.
func (s *ociImageSource) GetBlob(digest string) (io.ReadCloser, int64, error) {
	path, err := s.ref.blobPath(digest)
	if err != nil {
		return nil, 0, err
	}

	r, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	fi, err := r.Stat()
	if err != nil {
		return nil, 0, err
	}
	return r, fi.Size(), nil
}

func (s *ociImageSource) GetSignatures() ([][]byte, error) {
	return [][]byte{}, nil
}
