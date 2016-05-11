package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/projectatomic/skopeo/directory"
	"github.com/projectatomic/skopeo/types"
)

var (
	validHex = regexp.MustCompile(`^([a-f0-9]{64})$`)
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

type v1Image struct {
	// Config is the configuration of the container received from the client
	Config *config `json:"config,omitempty"`
	// DockerVersion specifies version on which image is built
	DockerVersion string `json:"docker_version,omitempty"`
	// Created timestamp when image was created
	Created time.Time `json:"created"`
	// Architecture is the hardware that the image is build and runs on
	Architecture string `json:"architecture,omitempty"`
	// OS is the operating system used to build and run the image
	OS string `json:"os,omitempty"`
}

// TODO(runcom)
func (i *dockerImage) DockerTar() ([]byte, error) {
	return nil, nil
}

// will support v1 one day...
type manifest interface {
	String() string
	GetLayers() []string
}

type manifestSchema1 struct {
	Name     string
	Tag      string
	FSLayers []struct {
		BlobSum string `json:"blobSum"`
	} `json:"fsLayers"`
	History []struct {
		V1Compatibility string `json:"v1Compatibility"`
	} `json:"history"`
	// TODO(runcom) verify the downloaded manifest
	//Signature []byte `json:"signature"`
}

func (m *manifestSchema1) GetLayers() []string {
	layers := make([]string, len(m.FSLayers))
	for i, layer := range m.FSLayers {
		layers[i] = layer.BlobSum
	}
	return layers
}

func (m *manifestSchema1) String() string {
	return fmt.Sprintf("%s-%s", sanitize(m.Name), sanitize(m.Tag))
}

func sanitize(s string) string {
	return strings.Replace(s, "/", "-", -1)
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

func fixManifestLayers(manifest *manifestSchema1) error {
	type imageV1 struct {
		ID     string
		Parent string
	}
	imgs := make([]*imageV1, len(manifest.FSLayers))
	for i := range manifest.FSLayers {
		img := &imageV1{}

		if err := json.Unmarshal([]byte(manifest.History[i].V1Compatibility), img); err != nil {
			return err
		}

		imgs[i] = img
		if err := validateV1ID(img.ID); err != nil {
			return err
		}
	}
	if imgs[len(imgs)-1].Parent != "" {
		return errors.New("Invalid parent ID in the base layer of the image.")
	}
	// check general duplicates to error instead of a deadlock
	idmap := make(map[string]struct{})
	var lastID string
	for _, img := range imgs {
		// skip IDs that appear after each other, we handle those later
		if _, exists := idmap[img.ID]; img.ID != lastID && exists {
			return fmt.Errorf("ID %+v appears multiple times in manifest", img.ID)
		}
		lastID = img.ID
		idmap[lastID] = struct{}{}
	}
	// backwards loop so that we keep the remaining indexes after removing items
	for i := len(imgs) - 2; i >= 0; i-- {
		if imgs[i].ID == imgs[i+1].ID { // repeated ID. remove and continue
			manifest.FSLayers = append(manifest.FSLayers[:i], manifest.FSLayers[i+1:]...)
			manifest.History = append(manifest.History[:i], manifest.History[i+1:]...)
		} else if imgs[i].Parent != imgs[i+1].ID {
			return fmt.Errorf("Invalid parent ID. Expected %v, got %v.", imgs[i+1].ID, imgs[i].Parent)
		}
	}
	return nil
}

func validateV1ID(id string) error {
	if ok := validHex.MatchString(id); !ok {
		return fmt.Errorf("image ID %q is invalid", id)
	}
	return nil
}
