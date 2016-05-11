package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
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

func (s *dockerImageSource) GetManifest() (manifest types.ImageManifest, unverifiedCanonicalDigest string, err error) {
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
	unverifiedCanonicalDigest = res.Header.Get("Docker-Content-Digest")
	manifest, err = makeImageManifest(s.ref.FullName(), manblob, unverifiedCanonicalDigest, nil)
	return
}

func (s *dockerImageSource) GetLayer(digest string) (io.ReadCloser, error) {
	url := fmt.Sprintf(blobsURL, s.ref.RemoteName(), digest)
	logrus.Infof("Downloading %s", url)
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

func makeImageManifest(name string, manblob []byte, dgst string, tagList []string) (types.ImageManifest, error) {
	m := manifestSchema1{}
	if err := json.Unmarshal(manblob, &m); err != nil {
		return nil, err
	}
	if err := fixManifestLayers(&m); err != nil {
		return nil, err
	}
	v1 := &v1Image{}
	if err := json.Unmarshal([]byte(m.History[0].V1Compatibility), v1); err != nil {
		return nil, err
	}
	return &types.DockerImageManifest{
		Name:          name,
		Tag:           m.Tag,
		Digest:        dgst,
		RepoTags:      tagList,
		DockerVersion: v1.DockerVersion,
		Created:       v1.Created,
		Labels:        v1.Config.Labels,
		Architecture:  v1.Architecture,
		Os:            v1.OS,
		FSLayers:      m.GetLayers(),
		RawManifest:   manblob,
	}, nil
}
