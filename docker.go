package main

import (
	"github.com/codegangsta/cli"
	"github.com/projectatomic/skopeo/docker"
)

type dockerImage struct {
	img *docker.DockerImage
}

func (i *dockerImage) GetManifest() ([]byte, error) {
	return i.img.GetManifest()
}

func (i *dockerImage) GetRawManifest(version string) ([]byte, error) {
	return i.img.GetRawManifest(version)
}

func (i *dockerImage) Kind() Kind {
	return KindDocker
}

func (i *dockerImage) GetLayers(layers []string) error {
	return i.img.GetLayers(layers)
}

func parseDockerImage(c *cli.Context, img string) (Image, error) {
	image, err := docker.ParseDockerImage(c, img)
	if err != nil {
		return nil, err
	}
	return &dockerImage{
		img: image,
	}, nil
}
