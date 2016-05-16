package docker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/projectatomic/skopeo/directory"
	"github.com/projectatomic/skopeo/types"
)

type dockerImage struct {
	src         *dockerImageSource
	digest      string
	rawManifest []byte
}

// ParseDockerImage returns a new Image interface type after setting up
// a client to the registry hosting the given image
func ParseDockerImage(img, certPath string, tlsVerify bool) (types.Image, error) {
	s, err := newDockerImageSource(img, certPath, tlsVerify)
	if err != nil {
		return nil, err
	}
	return &dockerImage{src: s}, nil
}

func (i *dockerImage) RawManifest(version string) ([]byte, error) {
	// TODO(runcom): unused version param for now, default to docker v2-1
	if err := i.retrieveRawManifest(); err != nil {
		return nil, err
	}
	return i.rawManifest, nil
}

func (i *dockerImage) Manifest() (types.ImageManifest, error) {
	if err := i.retrieveRawManifest(); err != nil {
		return nil, err
	}
	tags, err := i.getTags()
	if err != nil {
		return nil, err
	}
	imgManifest, err := makeImageManifest(i.src.ref.FullName(), i.rawManifest, i.digest, tags)
	if err != nil {
		return nil, err
	}
	return imgManifest, nil
}

func (i *dockerImage) getTags() ([]string, error) {
	// FIXME? Breaking the abstraction.
	url := fmt.Sprintf(tagsURL, i.src.ref.RemoteName())
	res, err := i.src.c.makeRequest("GET", url, nil, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		// print url also
		return nil, fmt.Errorf("Invalid status code returned when fetching tags list %d", res.StatusCode)
	}
	type tagsRes struct {
		Tags []string
	}
	tags := &tagsRes{}
	if err := json.NewDecoder(res.Body).Decode(tags); err != nil {
		return nil, err
	}
	return tags.Tags, nil
}

type config struct {
	Labels map[string]string
}

// TODO(runcom)
func (i *dockerImage) DockerTar() ([]byte, error) {
	return nil, nil
}

func (i *dockerImage) retrieveRawManifest() error {
	if i.rawManifest != nil {
		return nil
	}
	manifest, unverifiedCanonicalDigest, err := i.src.GetManifest()
	if err != nil {
		return err
	}
	i.rawManifest = manifest.Raw()
	i.digest = unverifiedCanonicalDigest
	return nil
}

func (i *dockerImage) getSchema1Manifest() (manifest, error) {
	if err := i.retrieveRawManifest(); err != nil {
		return nil, err
	}
	mschema1 := &manifestSchema1{}
	if err := json.Unmarshal(i.rawManifest, mschema1); err != nil {
		return nil, err
	}
	if err := fixManifestLayers(mschema1); err != nil {
		return nil, err
	}
	// TODO(runcom): verify manifest schema 1, 2 etc
	//if len(m.FSLayers) != len(m.History) {
	//return nil, fmt.Errorf("length of history not equal to number of layers for %q", ref.String())
	//}
	//if len(m.FSLayers) == 0 {
	//return nil, fmt.Errorf("no FSLayers in manifest for %q", ref.String())
	//}
	return mschema1, nil
}

func (i *dockerImage) Layers(layers ...string) error {
	m, err := i.getSchema1Manifest()
	if err != nil {
		return err
	}
	tmpDir, err := ioutil.TempDir(".", "layers-"+m.String()+"-")
	if err != nil {
		return err
	}
	dest := directory.NewDirImageDestination(tmpDir)
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := dest.PutManifest(data); err != nil {
		return err
	}
	if len(layers) == 0 {
		layers = m.GetLayers()
	}
	for _, l := range layers {
		if !strings.HasPrefix(l, "sha256:") {
			l = "sha256:" + l
		}
		if err := i.getLayer(dest, l); err != nil {
			return err
		}
	}
	return nil
}

func (i *dockerImage) getLayer(dest types.ImageDestination, digest string) error {
	stream, err := i.src.GetLayer(digest)
	if err != nil {
		return err
	}
	defer stream.Close()
	return dest.PutLayer(digest, stream)
}
