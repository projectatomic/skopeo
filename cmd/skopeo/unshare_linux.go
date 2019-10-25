package main

import (
	"github.com/containers/buildah/pkg/unshare"
	"github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/pkg/errors"
	"github.com/syndtr/gocapability/capability"
)

var neededCapabilities = []capability.Cap{
	capability.CAP_CHOWN,
	capability.CAP_DAC_OVERRIDE,
	capability.CAP_FOWNER,
	capability.CAP_FSETID,
	capability.CAP_MKNOD,
	capability.CAP_SETFCAP,
}

func maybeReexec() error {
	// With Skopeo we need only the subset of the root capabilities necessary
	// for pulling an image to the storage.  Do not attempt to create a namespace
	// if we already have the capabilities we need.
	capabilities, err := capability.NewPid(0)
	if err != nil {
		return errors.Wrapf(err, "error reading the current capabilities sets")
	}
	for _, cap := range neededCapabilities {
		if !capabilities.Get(capability.EFFECTIVE, cap) {
			// We miss a capability we need, create a user namespaces
			unshare.MaybeReexecUsingUserNamespace(true)
			return nil
		}
	}
	return nil
}

func reexecIfNecessaryForImages(imageNames ...string) error {
	// Check if container-storage are used before doing unshare
	for _, imageName := range imageNames {
		transport := alltransports.TransportFromImageName(imageName)
		if transport != nil && transport.Name() == storage.Transport.Name() {
			return maybeReexec()
		}
	}
	return nil
}
