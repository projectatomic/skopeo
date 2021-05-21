<img src="https://cdn.rawgit.com/containers/skopeo/master/docs/skopeo.svg" width="250">

----

# skopeoimage

## Overview

This directory contains the Dockerfiles necessary to create the skopeoimage container
images that are housed on quay.io under the skopeo account.  All repositories where
the images live are public and can be pulled without credentials.  These container images are secured and the
resulting containers can run safely with privileges within the container.

The container images are built using the latest Fedora and then Skopeo is installed into them.
The PATH in the container images is set to the default PATH provided by Fedora.  Also, the
ENTRYPOINT and the WORKDIR variables are not set within these container images, as such they
default to `/`.

The container images are:

  * `quay.io/containers/skopeo:<version>` and `quay.io/skopeo/stable:<version>` -
    These images are built when a new Skopeo version becomes available in
    Fedora.  These images are intended to be unchanging and stable, they will
    never be updated by automation once they've been pushed.  For build details,
    please [see the configuration file](stable/Dockerfile).
  * `quay.io/containers/skopeo:latest` and `quay.io/skopeo/stable:latest` -
    Built daily using the same Dockerfile as above.  The skopeo version
    will remain the "latest" available in Fedora, however the image
    contents may vary compared to the version-tagged images.
  * `quay.io/skopeo/testing:latest` - This image is built daily, using the
    latest version of Skopeo that was in the Fedora `updates-testing` repository.
    The image is Built with [the testing Dockerfile](testing/Dockerfile).
  * `quay.io/skopeo/upstream:latest` - This image is built daily using the latest
    code found in this GitHub repository.  Due to the image changing frequently,
    it's not guaranteed to be stable or even executable.  The image is built with
    [the upstream Dockerfile](upstream/Dockerfile).


## Sample Usage

Although not required, it is suggested that [Podman](https://github.com/containers/podman) be used with these container images.

```
# Get Help on Skopeo
podman run docker://quay.io/skopeo/stable:latest --help

# Get help on the Skopeo Copy command
podman run docker://quay.io/skopeo/stable:latest copy --help

# Copy the Skopeo container image from quay.io to
# a private registry
podman run docker://quay.io/skopeo/stable:latest copy docker://quay.io/skopeo/stable docker://registry.internal.company.com/skopeo

# Inspect the fedora:latest image
podman run docker://quay.io/skopeo/stable:latest inspect --config docker://registry.fedoraproject.org/fedora:latest  | jq
```
