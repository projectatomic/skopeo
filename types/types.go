package types

import (
	"fmt"
	"io"
	"time"
)

const (
	// AtomicPrefix is the URL-like schema prefix used for Atomic registry image references.
	AtomicPrefix = "atomic:"
	// DockerPrefix is the URL-like schema prefix used for Docker image references.
	DockerPrefix = "docker://"
	// DirectoryPrefix is the URL-like schema prefix used for local directories (for debugging)
	DirectoryPrefix = "dir:"
)

// Registry is a service providing repositories.
type Registry interface {
	Repositories() []Repository
	Repository(ref string) Repository
	Lookup(term string) []Image // docker registry v1 only AFAICT, v2 can be built hacking with Images()
}

// Repository is a set of images.
type Repository interface {
	Images() []Image
	Image(ref string) Image // ref == image name w/o registry part
}

// ImageSource is a service, possibly remote (= slow), to download components of a single image.
type ImageSource interface {
	GetManifest() (manifest ImageManifest, unverifiedCanonicalDigest string, err error)
	GetLayer(digest string) (io.ReadCloser, error)
	GetSignatures() ([][]byte, error)
}

// ImageDestination is a service, possibly remote (= slow), to store components of a single image.
type ImageDestination interface {
	// CanonicalDockerReference returns the full, unambiguous, Docker reference for this image (even if the user referred to the image using some shorthand notation).
	CanonicalDockerReference() (string, error)
	PutManifest([]byte) error
	PutLayer(digest string, stream io.Reader) error
	PutSignatures(signatures [][]byte) error
}

// Image is a Docker image in a repository.
type Image interface {
	// ref to repository?
	Layers(layers ...string) error // configure download directory? Call it DownloadLayers?
	Manifest() (ImageManifest, error)
	RawManifest(version string) ([]byte, error)
	DockerTar() ([]byte, error) // ??? also, configure output directory
}

// ImageManifest is the interesting subset of metadata about an Image.
// TODO(runcom)
type ImageManifest interface {
	Raw() []byte
	Layers() []string
	String() string
}

// DockerImageManifest is a set of metadata describing Docker images and their manifest.json files.
// Note that this is not exactly manifest.json, e.g. some fields have been added.
type DockerImageManifest struct {
	RawManifest   []byte `json:"-"`
	Name          string
	Tag           string
	Digest        string
	RepoTags      []string
	Created       time.Time
	DockerVersion string
	Labels        map[string]string
	Architecture  string
	Os            string
	FSLayers      []string `json:"Layers"`
}

// Raw returns the raw manifest as received from the registry
func (m *DockerImageManifest) Raw() []byte {
	return m.RawManifest
}

// Layers returns a list of the FS layers in the manifest
func (m *DockerImageManifest) Layers() []string {
	return m.FSLayers
}

func (m *DockerImageManifest) String() string {
	return fmt.Sprintf("%s:%s", m.Name, m.Tag)
}
