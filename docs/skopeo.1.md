% SKOPEO(1) Skopeo Man Pages
% Jhon Honce
% August 2016
# NAME
skopeo -- Various operations with container images images and container image registries
# SYNOPSIS
**skopeo** [_global options_] _command_ [_command options_]
# DESCRIPTION
`skopeo` is a command line utility providing various operations with container images and container image registries. For example, it is able to inspect a repository on a Docker registry and fetch image. It fetches the repository's manifest and it is able to show you a `docker inspect`-like json output about a whole repository or a tag. This tool, in contrast to `docker inspect`, helps you gather useful information about a repository or a tag without requiring you to run `docker pull` - e.g. - which tags are available for the given repository? which labels the image has?

It also allows you to copy container images between various registries, possibly converting them as necessary, and to sign and verify images.
## IMAGE NAMES
Most commands refer to container images, using a _transport_`:`_details_ format. The following formats are supported:

  **atomic:**_namespace_**/**_stream_**:**_tag_
  An image in the current project of the current default Atomic
  Registry. The current project and Atomic Registry instance are by
  default read from `$HOME/.kube/config`, which is set e.g. using
  `(oc login)`.

  **dir:**_path_
  An existing local directory _path_ storing the manifest, layer
  tarballs and signatures as individual files. This is a
  non-standardized format, primarily useful for debugging or
  noninvasive container inspection.

  **docker://**_docker-reference_
  An image in a registry implementing the "Docker Registry HTTP API V2".
  By default, uses the authorization state in `$HOME/.docker/config.json`,
  which is set e.g. using `(docker login)`.

  **oci:**_path_**:**_tag_
  An image _tag_ in a directory compliant with "Open Container Image
  Layout Specification" at _path_.

# OPTIONS

  **--debug** enable debug output

  **--policy** _path-to-policy_ Path to a policy.json file to use for verifying signatures and deciding whether an image is trusted, overriding the default trust policy file.

  **--registries.d** _dir_ use registry configuration files in _dir_ (e.g. for docker signature storage), overriding the default path.

  **--help**|**-h** Show help

  **--version**|**-v** print the version number

# COMMANDS

## skopeo copy
**skopeo copy** [**--sign-by=**_key-ID_] _source-image destination-image_

Copy an image (manifest, filesystem layers, signatures) from one location to another.

Uses the system's trust policy to validate images, rejects images not trusted by the policy.

  _source-image_ use the "image name" format described above

  _destination-image_ use the "image name" format described above

  **--remove-signatures** do not copy signatures, if any, from _source-image_. Necessary when copying a signed image to a destination which does not support signatures.

  **--sign-by=**_key-id_ add a signature using that key ID for an image name corresponding to _destination-image_

  **--src-creds** _username[:password]_ for accessing the source registry

  **--dest-creds** _username[:password]_ for accessing the destination registry

  **--src-cert-dir** _path_ Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the source registry

  **--src-tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to docker source registry (defaults to true)

  **--dest-cert-dir** _path_ Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the destination registry

  **--dest-tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to docker destination registry (defaults to true)

Existing signatures, if any, are preserved as well.

## skopeo delete
**skopeo delete** _image-name_

Mark _image-name_ for deletion.  To release the allocated disk space, you need to execute the docker registry garabage collector. E.g.,

```sh
$ docker exec -it registry bin/registry garbage-collect /etc/docker/registry/config.yml
```

  **--creds** _username[:password]_ for accessing the registry

  **--cert-dir** _path_ Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the registry

  **--tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to docker registries (defaults to true)

Additionally, the registry must allow deletions by setting `REGISTRY_STORAGE_DELETE_ENABLED=true` for the registry daemon.

## skopeo inspect
**skopeo inspect** [**--raw**] _image-name_

Return low-level information about _image-name_ in a registry

  **--raw** output raw manifest, default is to format in JSON

  _image-name_ name of image to retrieve information about

  **--creds** _username[:password]_ for accessing the registry

  **--cert-dir** _path_ Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the registry

  **--tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to docker registries (defaults to true)

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

## skopeo add-signature
**skopeo add-signature** [**--reference=**_signing-reference_] _image-reference key-fingerprint_

Add an additional signature to an image in its current location.  If only one
signature is needed, using `skopeo copy --sign-by` instead to publish and sign
an image in a single step is recommended over using `skopeo copy` followed by
`skopeo add-signature`.

  _reference_ A reference to the image

  _key-fingerprint_ Key identity to use for signing

  **--reference** Add a signature for the specified reference, instead of the image's default.

## skopeo verify-signature
**skopeo verify-signature** [**--reference=**_verify-reference_] _image-reference key-fingerprint_

Verify a signature on an image now.  Its digest will be printed on success.

  _reference_ A reference expected to identify the image

  _key-fingerprint_ Expected identity of the signing key

  **--reference** Verify a signature for the specified reference, instead of the image's default.

**Note:** If you do use this, make sure that the image can not be changed at the specified location between the times of its verification and use.

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

# AUTHORS

Antonio Murdaca <runcom@redhat.com>, Miloslav Trmac <mitr@redhat.com>, Jhon Honce <jhonce@redhat.com>

