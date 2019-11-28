skopeo [![Build Status](https://travis-ci.org/containers/skopeo.svg?branch=master)](https://travis-ci.org/containers/skopeo)
=

<img src="https://cdn.rawgit.com/containers/skopeo/master/docs/skopeo.svg" width="250">

----

`skopeo` is a command line utility that performs various operations on container images and image repositories.

`skopeo` can work with [OCI images](https://github.com/opencontainers/image-spec) as well as the original Docker v2 images.

Skopeo works with API V2 registries such as Docker registries, the Atomic registry, private registries, local directories and local OCI-layout directories.  Skopeo does not require a daemon to be running to perform these operations which consist of:

 * Copying an image from and to various storage mechanisms.
   For example you can copy images from one registry to another, without requiring privilege.
 * Inspecting a remote image showing its properties including its layers, without requiring you to pull the image to the host.
 * Deleting an image from an image repository.
 * When required by the repository, skopeo can pass the appropriate credentials and certificates for authentication.

 Skopeo operates on the following image and repository types:

 * containers-storage:docker-reference
         An image located in a local containers/storage image store.  Location and image store specified in /etc/containers/storage.conf

 * dir:path
         An existing local directory path storing the manifest, layer tarballs and signatures as individual files. This is a non-standardized format, primarily useful for debugging or noninvasive container inspection.

 * docker://docker-reference
         An image in a registry implementing the "Docker Registry HTTP API V2". By default, uses the authorization state in $HOME/.docker/config.json, which is set e.g. using (docker login).

 * docker-archive:path[:docker-reference]
         An image is stored in the `docker save` formated file.  docker-reference is only used when creating such a file, and it must not contain a digest.

 * docker-daemon:docker-reference
         An image docker-reference stored in the docker daemon internal storage.  docker-reference must contain either a tag or a digest.  Alternatively, when reading images, the format can also be docker-daemon:algo:digest (an image ID).

 * oci:path:tag
         An image tag in a directory compliant with "Open Container Image Layout Specification" at path.

Inspecting a repository
-
`skopeo` is able to _inspect_ a repository on a Docker registry and fetch images layers.
The _inspect_ command fetches the repository's manifest and it is able to show you a `docker inspect`-like
json output about a whole repository or a tag. This tool, in contrast to `docker inspect`, helps you gather useful information about
a repository or a tag before pulling it (using disk space).  The inspect command can show you which tags are available for the given 
repository, the labels the image has, the creation date and operating system of the image and more.  


Examples:
```sh
# show properties of fedora:latest
$ skopeo inspect docker://docker.io/fedora
{
    "Name": "docker.io/library/fedora",
    "Tag": "latest",
    "Digest": "sha256:cfd8f071bf8da7a466748f522406f7ae5908d002af1b1a1c0dcf893e183e5b32",
    "RepoTags": [
        "20",
        "21",
        "22",
        "23",
        "heisenbug",
        "latest",
        "rawhide"
    ],
    "Created": "2016-03-04T18:40:02.92155334Z",
    "DockerVersion": "1.9.1",
    "Labels": {},
    "Architecture": "amd64",
    "Os": "linux",
    "Layers": [
        "sha256:236608c7b546e2f4e7223526c74fc71470ba06d46ec82aeb402e704bfdee02a2",
        "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
    ]
}

# show unverifed image's digest
$ skopeo inspect docker://docker.io/fedora:rawhide | jq '.Digest'
"sha256:905b4846938c8aef94f52f3e41a11398ae5b40f5855fb0e40ed9c157e721d7f8"
```

Copying images
-
`skopeo` can copy container images between various storage mechanisms, including:
* Docker distribution based registries

  -  The Docker Hub, OpenShift, GCR, Artifactory, Quay ...

* Container Storage backends

  -  Docker daemon storage

  -  github.com/containers/storage (Backend for CRI-O, Buildah and friends)

* Local directories

* Local OCI-layout directories

```sh
$ skopeo copy docker://busybox:1-glibc atomic:myns/unsigned:streaming
$ skopeo copy docker://busybox:latest dir:existingemptydirectory
$ skopeo copy docker://busybox:latest oci:busybox_ocilayout:latest
```

Deleting images
-
For example,
```sh
$ skopeo delete docker://localhost:5000/imagename:latest
```

Private registries with authentication
-
When interacting with private registries, `skopeo` first looks for `--creds` (for `skopeo inspect|delete`) or `--src-creds|--dest-creds` (for `skopeo copy`) flags. If those aren't provided, it looks for the Docker's cli config file (usually located at `$HOME/.docker/config.json`) to get the credentials needed to authenticate. The ultimate fallback, as Docker does, is to provide an empty authentication when interacting with those registries.

Examples:
```sh
$ cat /home/runcom/.docker/config.json
{
	"auths": {
		"myregistrydomain.com:5000": {
			"auth": "dGVzdHVzZXI6dGVzdHBhc3N3b3Jk",
			"email": "stuf@ex.cm"
		}
	}
}

# we can see I'm already authenticated via docker login so everything will be fine
$ skopeo inspect docker://myregistrydomain.com:5000/busybox
{"Tag":"latest","Digest":"sha256:473bb2189d7b913ed7187a33d11e743fdc2f88931122a44d91a301b64419f092","RepoTags":["latest"],"Comment":"","Created":"2016-01-15T18:06:41.282540103Z","ContainerConfig":{"Hostname":"aded96b43f48","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":["/bin/sh","-c","#(nop) CMD [\"sh\"]"],"Image":"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":null},"DockerVersion":"1.8.3","Author":"","Config":{"Hostname":"aded96b43f48","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":["sh"],"Image":"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":null},"Architecture":"amd64","Os":"linux"}

# let's try now to fake a non existent Docker's config file
$ cat /home/runcom/.docker/config.json
{}

$ skopeo inspect docker://myregistrydomain.com:5000/busybox
FATA[0000] unauthorized: authentication required

# passing --creds - we can see that everything goes fine
$ skopeo inspect --creds=testuser:testpassword docker://myregistrydomain.com:5000/busybox
{"Tag":"latest","Digest":"sha256:473bb2189d7b913ed7187a33d11e743fdc2f88931122a44d91a301b64419f092","RepoTags":["latest"],"Comment":"","Created":"2016-01-15T18:06:41.282540103Z","ContainerConfig":{"Hostname":"aded96b43f48","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":["/bin/sh","-c","#(nop) CMD [\"sh\"]"],"Image":"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":null},"DockerVersion":"1.8.3","Author":"","Config":{"Hostname":"aded96b43f48","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":["sh"],"Image":"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":null},"Architecture":"amd64","Os":"linux"}

# skopeo copy example:
$ skopeo copy --src-creds=testuser:testpassword docker://myregistrydomain.com:5000/private oci:local_oci_image
```
If your cli config is found but it doesn't contain the necessary credentials for the queried registry
you'll get an error. You can fix this by either logging in (via `docker login`) or providing `--creds` or `--src-creds|--dest-creds`.


Obtaining skopeo
-
`skopeo` may already be packaged in your distribution, for example on Fedora 23 and later you can install it using
```sh
$ sudo dnf install skopeo
```
for openSUSE:
```sh
$ sudo zypper install skopeo
```
on alpine: 
```sh
$ sudo apk add skopeo
```


Otherwise, read on for building and installing it from source:

To build the `skopeo` binary you need at least Go 1.9.

There are two ways to build skopeo: in a container, or locally without a container.  Choose the one which better matches your needs and environment.

### Building without a container
Building without a container requires a bit more manual work and setup in your environment, but it is more flexible:
- It should work in more environments (e.g. for native macOS builds)
- It does not require root privileges (after dependencies are installed)
- It is faster, therefore more convenient for developing `skopeo`.

Install the necessary dependencies:
```sh
# Fedora:
sudo dnf install gpgme-devel libassuan-devel btrfs-progs-devel device-mapper-devel

# Ubuntu (`libbtrfs-dev` requires Ubuntu 18.10 and above):
sudo apt install libgpgme-dev libassuan-dev libbtrfs-dev libdevmapper-dev

# macOS:
brew install gpgme

# openSUSE
sudo zypper install libgpgme-devel device-mapper-devel libbtrfs-devel glib2-devel
```

Make sure to clone this repository in your `GOPATH` - otherwise compilation fails.

```sh
$ git clone https://github.com/containers/skopeo $GOPATH/src/github.com/containers/skopeo
$ cd $GOPATH/src/github.com/containers/skopeo && make binary-local
```

### Building in a container
Building in a container is simpler, but more restrictive:
- It requires the `docker` command and the ability to run Linux containers
- The created executable is a Linux executable, and depends on dynamic libraries which may only be available only in a container of a similar Linux distribution.

```sh
$ make binary # Or (make all) to also build documentation, see below.
```

To build a pure-Go static binary (disables devicemapper, btrfs, and gpgme):

```sh
$ make binary-static DISABLE_CGO=1
```

### Building documentation
To build the manual you will need go-md2man.
```sh
Debian$ sudo apt-get install go-md2man
Fedora$ sudo dnf install go-md2man
```
Then
```sh
$ make docs
```

### Installation
Finally, after the binary and documentation is built:
```sh
$ sudo make install
```

TODO
-
- list all images on registry?
- registry v2 search?
- show repo tags via flag or when reference isn't tagged or digested
- support rkt/appc image spec

NOT TODO
-
- provide a _format_ flag - just use the awesome [jq](https://stedolan.github.io/jq/)

CONTRIBUTING
-

Please read the [contribution guide](CONTRIBUTING.md) if you want to collaborate in the project.

License
-
skopeo is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE) for the full license text.
