package types

import (
	containerTypes "github.com/docker/engine-api/types/container"
)

const (
	// DockerPrefix is the URL-like schema prefix used for Docker image references.
	DockerPrefix = "docker://"
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

// Image is a Docker image in a repository.
type Image interface {
	// ref to repository?
	Layers(layers ...string) error // configure download directory? Call it DownloadLayers?
	Manifest(version string) (ImageManifest, error)
	RawManifest(version string) ([]byte, error)
	DockerTar() ([]byte, error) // ??? also, configure output directory
}

// ImageManifest is the interesting subset of metadata about an Image.
// TODO(runcom)
type ImageManifest interface {
	Labels() map[string]string
}

// DockerImageManifest is a set of metadata describing Docker images and their manifest.json files.
// Note that this is not exactly manifest.json, e.g. some fields have been added.
type DockerImageManifest struct {
	Tag             string
	Digest          string
	RepoTags        []string
	Comment         string
	Created         string
	ContainerConfig *containerTypes.Config // remove docker/docker code, this isn't needed
	DockerVersion   string
	Author          string
	Config          *containerTypes.Config // remove docker/docker code, needs just Labels here for now, maybe Cmd? Hostname?
	Architecture    string
	Os              string
	Layers          []string // ???
}

// Labels returns labels attached to this image.
func (m *DockerImageManifest) Labels() map[string]string {
	return m.Config.Labels
}
