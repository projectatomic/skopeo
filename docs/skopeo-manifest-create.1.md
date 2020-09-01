% skopeo-manifest-create(1)

## NAME
skopeo\-manifest\-create -Create a manifest list and write it to standard output.

## SYNOPSIS
**skopeo manifest create** _image(s)_

## DESCRIPTION

Create a manifest list based on the images specified and write it to the standard out.

This command can be used to create manifest files for multi architecture images.

## OPTIONS
**--authfile** _path_

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `skopeo login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

**--creds** _username[:password]_ for accessing the registry.

**--cert-dir** _path_ Use certificates (*.crt, *.cert, *.key) at _path_ to connect to the registry or daemon.

**--no-creds** _bool-value_ Access the registry anonymously.

**--tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to a container registry or daemon (defaults to true).

## EXAMPLES

```sh
$ skopeo manifest create docker://registry.local.lan/hello:0.0.1-amd64 docker://registry.local.lan/hello:0.0.1-arm64 | jq
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
  "manifests": [
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "size": 1158,
      "digest": "sha256:fe870de8c9f2eca2ffae48c12d45b5d491bf0f69fe14762c78faa8a4a1931e81",
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "size": 1158,
      "digest": "sha256:f148b87db4fd4d775ed8e7dddd09b3dbc0bf4831b77a670feea1ba69678c563c",
      "platform": {
        "architecture": "arm64",
        "os": "linux"
      }
    }
  ]
}
```

In this example we create a multi-architecture manifest referncing two images that are inside of the `registry.local.lan`.
The manifest will reference `hello:0.0.1-amd64` as the Linux x86_64 image, while it will reference `hello:0.0.1-arm64` as the Linux ARM64 image to be used.

## SEE ALSO
skopeo(1) skopeo-manifest-push(1)

## AUTHORS

Flavio Castelli <fcastelli@suse.com>

