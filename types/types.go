package types

import (
	containerTypes "github.com/docker/engine-api/types/container"
)

// ImageInspect describes a generic image
type ImageInspect struct {
	Tag             string
	Digest          string
	RawManifest     []byte
	RepoTags        []string
	Comment         string
	Created         string
	ContainerConfig *containerTypes.Config
	DockerVersion   string
	Author          string
	Config          *containerTypes.Config
	Architecture    string
	Os              string
}
