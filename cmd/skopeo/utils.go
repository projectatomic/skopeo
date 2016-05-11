package main

import (
	"fmt"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/projectatomic/skopeo/directory"
	"github.com/projectatomic/skopeo/docker"
	"github.com/projectatomic/skopeo/openshift"
	"github.com/projectatomic/skopeo/types"
)

// ParseImage converts image URL-like string to an initialized handler for that image.
func parseImage(c *cli.Context) (types.Image, error) {
	var (
		imgName   = c.Args().First()
		certPath  = c.GlobalString("cert-path")
		tlsVerify = c.GlobalBool("tls-verify")
	)
	switch {
	case strings.HasPrefix(imgName, types.DockerPrefix):
		return docker.ParseDockerImage(strings.TrimPrefix(imgName, types.DockerPrefix), certPath, tlsVerify)
		//case strings.HasPrefix(img, appcPrefix):
		//
	}
	return nil, fmt.Errorf("no valid prefix provided")
}

// parseImageSource converts image URL-like string to an ImageSource.
func parseImageSource(c *cli.Context, name string) (types.ImageSource, error) {
	var (
		certPath  = c.GlobalString("cert-path")
		tlsVerify = c.GlobalBool("tls-verify") // FIXME!! defaults to false?
	)
	switch {
	case strings.HasPrefix(name, types.DockerPrefix):
		return docker.NewDockerImageSource(strings.TrimPrefix(name, types.DockerPrefix), certPath, tlsVerify)
	case strings.HasPrefix(name, types.AtomicPrefix):
		return openshift.NewOpenshiftImageSource(strings.TrimPrefix(name, types.AtomicPrefix), certPath, tlsVerify)
	case strings.HasPrefix(name, types.DirectoryPrefix):
		return directory.NewDirImageSource(strings.TrimPrefix(name, types.DirectoryPrefix)), nil
	}
	return nil, fmt.Errorf("Unrecognized image reference %s", name)
}

// parseImageDestination converts image URL-like string to an ImageDestination.
func parseImageDestination(c *cli.Context, name string) (types.ImageDestination, error) {
	var (
		certPath  = c.GlobalString("cert-path")
		tlsVerify = c.GlobalBool("tls-verify") // FIXME!! defaults to false?
	)
	switch {
	case strings.HasPrefix(name, types.DockerPrefix):
		return docker.NewDockerImageDestination(strings.TrimPrefix(name, types.DockerPrefix), certPath, tlsVerify)
	case strings.HasPrefix(name, types.AtomicPrefix):
		return openshift.NewOpenshiftImageDestination(strings.TrimPrefix(name, types.AtomicPrefix), certPath, tlsVerify)
	case strings.HasPrefix(name, types.DirectoryPrefix):
		return directory.NewDirImageDestination(strings.TrimPrefix(name, types.DirectoryPrefix)), nil
	}
	return nil, fmt.Errorf("Unrecognized image reference %s", name)
}
