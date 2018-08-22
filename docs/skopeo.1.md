% SKOPEO(1) Skopeo Man Pages
% Jhon Honce
% August 2016
# NAME
skopeo -- Command line utility used to interact with local and remote container images and container image registries
# SYNOPSIS
**skopeo** [_global options_] _command_ [_command options_]
# DESCRIPTION
`skopeo` is a command line utility providing various operations with container images and container image registries.

`skopeo` can copy container images between various containers image stores, converting them as necessary.  For example you can use `skopeo` to copy container images from one container registry to another.

`skopeo` can convert a Docker schema 2 or schema 1 container image to an OCI image.

`skopeo` can inspect a repository on a container registry without needlessly pulling the image. Pulling an image from a repository, especially a remote repository, is an expensive network and storage operation. Skopeo fetches the repository's manifest and displays a `docker inspect`-like json output about the repository or a tag. `skopeo`, in contrast to `docker inspect`, helps you gather useful information about a repository or a tag without requiring you to run `docker pull` - e.g. - Which tags are available for the given repository? Which labels does the image have?

`skopeo` can sign and verify container images.

`skopeo` can delete container images from a remote container registry.

Note: `skopeo` does not require any container runtimes to be running, to do most of
its functionality.  It also does not require root, unless you are copying images into a container runtime storage backend, like the docker daemon or github.com/containers/storage.

## IMAGE NAMES
Most commands refer to container images, using a _transport_`:`_details_ format. The following formats are supported:

  **containers-storage:**_docker-reference_
  An image located in a local containers/storage image store.  Location and image store specified in /etc/containers/storage.conf

  **dir:**_path_
  An existing local directory _path_ storing the manifest, layer tarballs and signatures as individual files. This is a non-standardized format, primarily useful for debugging or noninvasive container inspection.

  **docker://**_docker-reference_
  An image in a registry implementing the "Docker Registry HTTP API V2". By default, uses the authorization state in either `$XDG_RUNTIME_DIR/containers/auth.json`, which is set using `(kpod login)`. If the authorization state is not found there, `$HOME/.docker/config.json` is checked, which is set using `(docker login)`.

  **docker-archive:**_path_[**:**_docker-reference_]
  An image is stored in the `docker save` formatted file.  _docker-reference_ is only used when creating such a file, and it must not contain a digest.

  **docker-daemon:**_docker-reference_
  An image _docker-reference_ stored in the docker daemon internal storage.  _docker-reference_ must contain either a tag or a digest.  Alternatively, when reading images, the format can be docker-daemon:algo:digest (an image ID).

  **oci:**_path_**:**_tag_
  An image _tag_ in a directory compliant with "Open Container Image Layout Specification" at _path_.

  **ostree:**_image_[**@**_/absolute/repo/path_]
  An image in local OSTree repository.  _/absolute/repo/path_ defaults to _/ostree/repo_.

# OPTIONS

  **--debug** enable debug output

  **--policy** _path-to-policy_ Path to a policy.json file to use for verifying signatures and deciding whether an image is trusted, overriding the default trust policy file.

  **--insecure-policy** Adopt an insecure, permissive policy that allows anything. This obviates the need for a policy file.

  **--registries.d** _dir_ use registry configuration files in _dir_ (e.g. for container signature storage), overriding the default path.

  **--override-arch** _arch_ Use _arch_ instead of the architecture of the machine for choosing images.

  **--override-os** _OS_ Use _OS_ instead of the running OS for choosing images.

  **--command-timeout** _duration_ Timeout for the command execution.

  **--help**|**-h** Show help

  **--version**|**-v** print the version number

# COMMANDS

## skopeo copy
**skopeo copy** [**--sign-by=**_key-ID_] _source-image destination-image_

Copy an image (manifest, filesystem layers, signatures) from one location to another.

Uses the system's trust policy to validate images, rejects images not trusted by the policy.

  _source-image_ use the "image name" format described above

  _destination-image_ use the "image name" format described above

  **--authfile** _path_

  Path of the authentication file. Default is ${XDG_RUNTIME\_DIR}/containers/auth.json, which is set using `kpod login`.
  If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

  **--format, -f** _manifest-type_ Manifest type (oci, v2s1, or v2s2) to use when saving image to directory using the 'dir:' transport (default is manifest type of source)

  **--remove-signatures** do not copy signatures, if any, from _source-image_. Necessary when copying a signed image to a destination which does not support signatures.

  **--sign-by=**_key-id_ add a signature using that key ID for an image name corresponding to _destination-image_

  **--src-creds** _username[:password]_ for accessing the source registry

  **--dest-compress** _bool-value_ Compress tarball image layers when saving to directory using the 'dir' transport. (default is same compression type as source)

  **--dest-creds** _username[:password]_ for accessing the destination registry

  **--src-cert-dir** _path_ Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the source registry or daemon

  **--src-tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to container source registry or daemon (defaults to true)

  **--dest-cert-dir** _path_ Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the destination registry or daemon

  **--dest-ostree-tmp-dir** _path_ Directory to use for OSTree temporary files.

  **--dest-tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to container destination registry or daemon (defaults to true)

  **--src-daemon-host** _host_ Copy from docker daemon at _host_. If _host_ starts with `tcp://`, HTTPS is enabled by default. To use plain HTTP, use the form `http://` (default is `unix:///var/run/docker.sock`).

  **--dest-daemon-host** _host_ Copy to docker daemon at _host_. If _host_ starts with `tcp://`, HTTPS is enabled by default. To use plain HTTP, use the form `http://` (default is `unix:///var/run/docker.sock`).

   **--try-torrent** Try to fetch the layers using BitTorrent.

   **--torrent-trackers** _tracker_ Specify a tracker to use with BitTorrent.  Can be specified multiple times.  If set, it overrides the tracker information from the BitTorrent file.

Existing signatures, if any, are preserved as well.

## skopeo delete
**skopeo delete** _image-name_

Mark _image-name_ for deletion.  To release the allocated disk space, you must login to the container registry server and execute the container registry garbage collector. E.g.,

```
/usr/bin/registry garbage-collect /etc/docker-distribution/registry/config.yml

Note: sometimes the config.yml is stored in /etc/docker/registry/config.yml

If you are running the container registry inside of a container you would execute something like:

$ docker exec -it registry /usr/bin/registry garbage-collect /etc/docker-distribution/registry/config.yml

```

  **--authfile** _path_

  Path of the authentication file. Default is ${XDG_RUNTIME\_DIR}/containers/auth.json, which is set using `kpod login`.
  If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

  **--creds** _username[:password]_ for accessing the registry

  **--cert-dir** _path_ Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the registry

  **--tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to container registries (defaults to true)

Additionally, the registry must allow deletions by setting `REGISTRY_STORAGE_DELETE_ENABLED=true` for the registry daemon.

## skopeo inspect
**skopeo inspect** [**--raw**] _image-name_

Return low-level information about _image-name_ in a registry

  **--raw** output raw manifest, default is to format in JSON

  _image-name_ name of image to retrieve information about

  **--authfile** _path_

  Path of the authentication file. Default is ${XDG_RUNTIME\_DIR}/containers/auth.json, which is set using `kpod login`.
  If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

  **--creds** _username[:password]_ for accessing the registry

  **--cert-dir** _path_ Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the registry

  **--tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to container registries (defaults to true)

## skopeo manifest-digest
**skopeo manifest-digest** _manifest-file_

Compute a manifest digest of _manifest-file_ and write it to standard output.

## skopeo standalone-sign
**skopeo standalone-sign** _manifest docker-reference key-fingerprint_ **--output**|**-o** _signature_

This is primarily a debugging tool, or useful for special cases,
and usually should not be a part of your normal operational workflow; use `skopeo copy --sign-by` instead to publish and sign an image in one step.

  _manifest_ Path to a file containing the image manifest

  _docker-reference_ A docker reference to identify the image with

  _key-fingerprint_ Key identity to use for signing

  **--output**|**-o** output file

## skopeo standalone-verify
**skopeo standalone-verify** _manifest docker-reference key-fingerprint signature_

Verify a signature using local files, digest will be printed on success.

  _manifest_ Path to a file containing the image manifest

  _docker-reference_ A docker reference expected to identify the image in the signature

  _key-fingerprint_ Expected identity of the signing key

  _signature_ Path to signature file

**Note:** If you do use this, make sure that the image can not be changed at the source location between the times of its verification and use.

## skopeo help
show help for `skopeo`

# FILES
  **/etc/containers/policy.json**
  Default trust policy file, if **--policy** is not specified.
  The policy format is documented in https://github.com/containers/image/blob/master/docs/policy.json.md .

  **/etc/containers/registries.d**
  Default directory containing registry configuration, if **--registries.d** is not specified.
  The contents of this directory are documented in https://github.com/containers/image/blob/master/docs/registries.d.md .

# EXAMPLES

## skopeo copy
To copy the layers of the docker.io busybox image to a local directory:
```sh
$ mkdir -p /var/lib/images/busybox
$ skopeo copy docker://busybox:latest dir:/var/lib/images/busybox
$ ls /var/lib/images/busybox/*
  /tmp/busybox/2b8fd9751c4c0f5dd266fcae00707e67a2545ef34f9a29354585f93dac906749.tar
  /tmp/busybox/manifest.json
  /tmp/busybox/8ddc19f16526912237dd8af81971d5e4dd0587907234be2b83e249518d5b673f.tar
```

To copy and sign an image:

```sh
$ skopeo copy --sign-by dev@example.com atomic:example/busybox:streaming atomic:example/busybox:gold
```
## skopeo delete
Mark image example/pause for deletion from the registry.example.com registry:
```sh
$ skopeo delete --force docker://registry.example.com/example/pause:latest
```
See above for additional details on using the command **delete**.

## skopeo inspect
To review information for the image fedora from the docker.io registry:
```sh
$ skopeo inspect docker://docker.io/fedora
{
    "Name": "docker.io/library/fedora",
    "Digest": "sha256:a97914edb6ba15deb5c5acf87bd6bd5b6b0408c96f48a5cbd450b5b04509bb7d",
    "RepoTags": [
        "20",
        "21",
        "22",
        "23",
        "24",
        "heisenbug",
        "latest",
        "rawhide"
    ],
    "Created": "2016-06-20T19:33:43.220526898Z",
    "DockerVersion": "1.10.3",
    "Labels": {},
    "Architecture": "amd64",
    "Os": "linux",
    "Layers": [
        "sha256:7c91a140e7a1025c3bc3aace4c80c0d9933ac4ee24b8630a6b0b5d8b9ce6b9d4"
    ]
}
```
## skopeo layers
Another method to retrieve the layers for the busybox image from the docker.io registry:
```sh
$ skopeo layers docker://busybox
$ ls layers-500650331/
  8ddc19f16526912237dd8af81971d5e4dd0587907234be2b83e249518d5b673f.tar
  manifest.json
  a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4.tar
```
## skopeo manifest-digest
```sh
$ skopeo manifest-digest manifest.json
sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6
```
## skopeo standalone-sign
```sh
$ skopeo standalone-sign busybox-manifest.json registry.example.com/example/busybox 1D8230F6CDB6A06716E414C1DB72F2188BB46CC8 --output busybox.signature
$
```

See `skopeo copy` above for the preferred method of signing images.
## skopeo standalone-verify
```sh
$ skopeo standalone-verify busybox-manifest.json registry.example.com/example/busybox 1D8230F6CDB6A06716E414C1DB72F2188BB46CC8  busybox.signature
Signature verified, digest sha256:20bf21ed457b390829cdbeec8795a7bea1626991fda603e0d01b4e7f60427e55
```

# SEE ALSO
kpod-login(1), docker-login(1)

# AUTHORS

Antonio Murdaca <runcom@redhat.com>, Miloslav Trmac <mitr@redhat.com>, Jhon Honce <jhonce@redhat.com>

