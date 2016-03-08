skopeo [![Build Status](https://travis-ci.org/projectatomic/skopeo.svg?branch=master)](https://travis-ci.org/projectatomic/skopeo)
=

_Please be aware `skopeo` is still work in progress_

`skopeo` is a command line utility which is able to _inspect_ a repository on a Docker registry and fetch an image.
By _inspect_ I mean it fetches the repository's manifest and it is able to show you a `docker inspect`-like
json output about a whole repository or a tag. This tool, in contrast to `docker inspect`, helps you gather useful information about
a repository or a tag before pulling it (using disk space) - e.g. - which tags are available for the given repository? which labels the image has?

Examples:
```sh
# show repository's labels of rhel7:latest
$ skopeo inspect registry.access.redhat.com/rhel7 | jq '.Config.Labels'
{
  "Architecture": "x86_64",
  "Authoritative_Registry": "registry.access.redhat.com",
  "BZComponent": "rhel-server-docker",
  "Build_Host": "rcm-img04.build.eng.bos.redhat.com",
  "Name": "rhel7/rhel",
  "Release": "38",
  "Vendor": "Red Hat, Inc.",
  "Version": "7.2"
}

# show repository's tags
$ skopeo inspect docker.io/fedora | jq '.RepoTags'
[
  "20",
  "21",
  "22",
  "23",
  "heisenbug",
  "latest",
  "rawhide"
]

# show image's digest
$ skopeo inspect docker.io/fedora:rawhide | jq '.Digest'
"sha256:905b4846938c8aef94f52f3e41a11398ae5b40f5855fb0e40ed9c157e721d7f8"

# fetch the layers for docker.io/fedora:rawhide
$ skopeo layers docker://docker.io/fedora:rawhide
$ ls
layers-library-fedora-rawhide-180927735

```

Private registries with authentication
-
When interacting with private registries, `skopeo` first looks for the Docker's cli config file (usually located at `$HOME/.docker/config.json`) to get the credentials needed to authenticate. When the file isn't available it falls back looking for `--username` and `--password` flags. The ultimate fallback, as Docker does, is to provide an empty authentication when interacting with those registries.

Examples:
```sh
# on my system
$ skopeo --help | grep docker-cfg
   --docker-cfg "/home/runcom/.docker"	Docker's cli config for auth

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
$ skopeo inspect myregistrydomain.com:5000/busybox
{"Tag":"latest","Digest":"sha256:473bb2189d7b913ed7187a33d11e743fdc2f88931122a44d91a301b64419f092","RepoTags":["latest"],"Comment":"","Created":"2016-01-15T18:06:41.282540103Z","ContainerConfig":{"Hostname":"aded96b43f48","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":["/bin/sh","-c","#(nop) CMD [\"sh\"]"],"Image":"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":null},"DockerVersion":"1.8.3","Author":"","Config":{"Hostname":"aded96b43f48","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":["sh"],"Image":"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":null},"Architecture":"amd64","Os":"linux"}

# let's try now to fake a non existent Docker's config file
$ skopeo --docker-cfg="" myregistrydomain.com:5000/busybox
FATA[0000] Get https://myregistrydomain.com:5000/v2/busybox/manifests/latest: no basic auth credentials

# passing --username and --password - we can see that everything goes fine
$ skopeo --docker-cfg="" --username=testuser --password=testpassword myregistrydomain.com:5000/busybox
{"Tag":"latest","Digest":"sha256:473bb2189d7b913ed7187a33d11e743fdc2f88931122a44d91a301b64419f092","RepoTags":["latest"],"Comment":"","Created":"2016-01-15T18:06:41.282540103Z","ContainerConfig":{"Hostname":"aded96b43f48","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":["/bin/sh","-c","#(nop) CMD [\"sh\"]"],"Image":"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":null},"DockerVersion":"1.8.3","Author":"","Config":{"Hostname":"aded96b43f48","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":["sh"],"Image":"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":null},"Architecture":"amd64","Os":"linux"}
```
If your cli config is found but it doesn't contain the necessary credentials for the queried registry
you'll get an error. You can fix this by either logging in (via `docker login`) or providing `--username`
and `--password`.
Building
-
To build `skopeo` you need at least Go 1.5 because it uses the latest `GO15VENDOREXPERIMENT` flag. Also, make sure to clone the repository in your `GOPATH` - otherwise compilation fails.
```sh
$ cd $GOPATH/src
$ mkdir -p github.com/projectatomic
$ cd projectatomic
$ git clone https://github.com/projectatomic/skopeo
$ cd skopeo && make binary
```
Man:
-
To build the man page you need [`go-md2man`](https://github.com/cpuguy83/go-md2man) available on your system, then:
```
$ make man
```
Installing
-
If you built from source:
```sh
$ sudo make install
```
`skopeo` is also available from Fedora 23:
```sh
sudo dnf install skopeo
```
Tests
-
_You need Docker installed on your system in order to run the test suite_
```sh
$ make test-integration
```
TODO
-
- make skopeo docker registry v2 only
- output raw manifest
- download layers and support docker load tar(s)
- get rid of docker/docker code (?)
- show repo tags via flag or when reference isn't tagged or digested
- add tests (integration with deployed registries in container - Docker-like)
- support rkt/appc image spec

NOT TODO
-
- provide a _format_ flag - just use the awesome [jq](https://stedolan.github.io/jq/)

License
-
ASL 2.0
