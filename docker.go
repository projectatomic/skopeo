package main

import (
	"github.com/codegangsta/cli"
	"github.com/projectatomic/skopeo/docker"
)

type dockerImage struct {
	img *docker.Image
}

func (i *dockerImage) GetManifest() ([]byte, error) {
	return i.img.GetManifest()
}

func (i *dockerImage) GetRawManifest(version string) ([]byte, error) {
	return i.img.GetRawManifest(version)
}

func (i *dockerImage) Kind() kind {
	return kindDocker
}

func (i *dockerImage) GetLayers(layers []string) error {
	return i.img.GetLayers(layers)
}

func parseDockerImage(c *cli.Context, img string) (image, error) {
	image, err := docker.ParseImage(c, img)
	if err != nil {
		return nil, err
	}
	return &dockerImage{
		img: image,
	}, nil
}
