% skopeo-manifest-push(1)

## NAME
skopeo\-manifest\-push -Push a manifest list to a remote container registry.

## SYNOPSIS
**skopeo manifest push** _dest-image_

## DESCRIPTION

Read a manifest file from stdin and push it to the specified location.

This command can be used to push a manifest file describing multi architecture images.

*Note well:* the images refereneced by the manifest must already exist inside of the remote registry.

## OPTIONS
**--authfile** _path_

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `skopeo login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

**--creds** _username[:password]_ for accessing the registry.

**--cert-dir** _path_ Use certificates (*.crt, *.cert, *.key) at _path_ to connect to the registry or daemon.

**--input** _path_ Read manifest from the specified file instead of STDIN.

**--no-creds** _bool-value_ Access the registry anonymously.

**--tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to a container registry or daemon (defaults to true).

## EXAMPLES

```sh
$ skopeo manifest create docker://registry.local.lan/hello:0.0.1-amd64 docker://registry.local.lan/hello:0.0.1-arm64 | skopeo manifest push docker://registry.local.lan/hello:0.0.1
Pushing manifest
Manifest successfully pushed
```

In this example we create a multi-architecture manifest by leveraging the `skopeo manifest create` command.
The output produced by this command is then consumed by the `skopeo manifest push` command.

A new manifest list is created on the `registry.local.lan` that is identified as `hello:0.0.1`.
Users can pull the `registry.local.lan/hello:0.0.1` image and have the container engine automatically obtain the right (architecture and OS wise) image to be used.

## SEE ALSO
skopeo(1) skopeo-manifest-create(1)

## AUTHORS

Flavio Castelli <fcastelli@suse.com>

