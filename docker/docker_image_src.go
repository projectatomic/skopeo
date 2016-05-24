package docker

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/projectatomic/skopeo/docker/utils"
	"github.com/projectatomic/skopeo/reference"
	"github.com/projectatomic/skopeo/types"
)

type errFetchManifest struct {
	statusCode int
	body       []byte
}

func (e errFetchManifest) Error() string {
	return fmt.Sprintf("error fetching manifest: status code: %d, body: %s", e.statusCode, string(e.body))
}

type dockerImageSource struct {
	ref reference.Named
	tag string
	c   *dockerClient
}

// newDockerImageSource is the same as NewDockerImageSource, only it returns the more specific *dockerImageSource type.
func newDockerImageSource(img, certPath string, tlsVerify bool) (*dockerImageSource, error) {
	ref, tag, err := parseDockerImageName(img)
	if err != nil {
		return nil, err
	}
	c, err := newDockerClient(ref.Hostname(), certPath, tlsVerify)
	if err != nil {
		return nil, err
	}
	return &dockerImageSource{
		ref: ref,
		tag: tag,
		c:   c,
	}, nil
}

// NewDockerImageSource creates a new ImageSource for the specified image and connection specification.
func NewDockerImageSource(img, certPath string, tlsVerify bool) (types.ImageSource, error) {
	return newDockerImageSource(img, certPath, tlsVerify)
}

// IntendedDockerReference returns the full, unambiguous, Docker reference for this image, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
// May be "" if unknown.
func (s *dockerImageSource) IntendedDockerReference() string {
	return fmt.Sprintf("%s:%s", s.ref.Name(), s.tag)
}

// ManifestMIMETypes returns a slice of supported MIME types
func manifestMIMETypes() []string {
	return []string{
		utils.DockerV2Schema1MIMEType,
		utils.DockerV2Schema2MIMEType,
		utils.DockerV2ListMIMEType,
	}
}

func (s *dockerImageSource) GetManifest() ([]byte, string, error) {
	url := fmt.Sprintf(manifestURL, s.ref.RemoteName(), s.tag)
	// TODO(runcom) set manifest version header! schema1 for now - then schema2 etc etc and v1
	// TODO(runcom) NO, switch on the resulter manifest like Docker is doing
	res, err := s.c.makeRequest("GET", url, nil, nil)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	manblob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	if res.StatusCode != http.StatusOK {
		return nil, "", errFetchManifest{res.StatusCode, manblob}
	}
	// We might validate manblob against the Docker-Content-Digest header here to protect against transport errors.
	return manblob, res.Header.Get("Content-Type"), nil
}

func (s *dockerImageSource) GetLayer(digest string) (io.ReadCloser, error) {
	url := fmt.Sprintf(blobsURL, s.ref.RemoteName(), digest)
	res, err := s.c.makeRequest("GET", url, nil, nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		// print url also
		return nil, fmt.Errorf("Invalid status code returned when fetching blob %d", res.StatusCode)
	}
	return res.Body, nil
}

func (s *dockerImageSource) GetSignatures() ([][]byte, error) {
	return [][]byte{}, nil
}
