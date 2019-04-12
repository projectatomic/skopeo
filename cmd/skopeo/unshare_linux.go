package main

import "github.com/containers/buildah/pkg/unshare"

func maybeReexec() {
	unshare.MaybeReexecUsingUserNamespace(false)
}
