package main

import (
	"fmt"
	"strings"

	"github.com/projectatomic/skopeo/directory"
	"github.com/projectatomic/skopeo/docker"
	"github.com/projectatomic/skopeo/image"
	"github.com/projectatomic/skopeo/openshift"
	"github.com/projectatomic/skopeo/types"
	"github.com/urfave/cli"
)

const (
	// atomicPrefix is the URL-like schema prefix used for Atomic registry image references.
	atomicPrefix = "atomic:"
	// dockerPrefix is the URL-like schema prefix used for Docker image references.
	dockerPrefix = "docker://"
	// directoryPrefix is the URL-like schema prefix used for local directories (for debugging)
	directoryPrefix = "dir:"
)

// ParseImage converts image URL-like string to an initialized handler for that image.
func parseImage(c *cli.Context) (types.Image, error) {
	var (
		imgName   = c.Args().First()
		certPath  = c.GlobalString("cert-path")
		tlsVerify = c.GlobalBool("tls-verify")
	)
	switch {
	case strings.HasPrefix(imgName, dockerPrefix):
		return docker.NewDockerImage(strings.TrimPrefix(imgName, dockerPrefix), certPath, tlsVerify)
		//case strings.HasPrefix(img, appcPrefix):
		//
	case strings.HasPrefix(imgName, directoryPrefix):
		src := directory.NewDirImageSource(strings.TrimPrefix(imgName, directoryPrefix))
		return image.FromSource(src), nil
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
	case strings.HasPrefix(name, dockerPrefix):
		return docker.NewDockerImageSource(strings.TrimPrefix(name, dockerPrefix), certPath, tlsVerify)
	case strings.HasPrefix(name, atomicPrefix):
		return openshift.NewOpenshiftImageSource(strings.TrimPrefix(name, atomicPrefix), certPath, tlsVerify)
	case strings.HasPrefix(name, directoryPrefix):
		return directory.NewDirImageSource(strings.TrimPrefix(name, directoryPrefix)), nil
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
	case strings.HasPrefix(name, dockerPrefix):
		return docker.NewDockerImageDestination(strings.TrimPrefix(name, dockerPrefix), certPath, tlsVerify)
	case strings.HasPrefix(name, atomicPrefix):
		return openshift.NewOpenshiftImageDestination(strings.TrimPrefix(name, atomicPrefix), certPath, tlsVerify)
	case strings.HasPrefix(name, directoryPrefix):
		return directory.NewDirImageDestination(strings.TrimPrefix(name, directoryPrefix)), nil
	}
	return nil, fmt.Errorf("Unrecognized image reference %s", name)
}
