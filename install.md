# Installing from packages

`skopeo` may already be packaged in your distribution, for example on
RHEL/CentOS ≥ 8 or Fedora you can install it using:

```sh
$ sudo dnf install skopeo
```

on RHEL/CentOS ≤ 7.x:

```sh
$ sudo yum install skopeo
```

for openSUSE:

```sh
$ sudo zypper install skopeo
```

on alpine:

```sh
$ sudo apk add skopeo
```

Debian (10 and newer including Raspbian) and Ubuntu (18.04 and newer): Packages
are available via the [Kubic][0] project repositories:

[0]: https://build.opensuse.org/project/show/devel:kubic:libcontainers:stable

```bash
# Debian Unstable/Sid:
$ echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_Unstable/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
$ wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/Debian_Unstable/Release.key -O- | sudo apt-key add -
```

```bash
# Debian Testing:
$ echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_Testing/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
$ wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/Debian_Testing/Release.key -O- | sudo apt-key add -
```

```bash
# Debian 10:
$ echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_10/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
$ wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/Debian_10/Release.key -O- | sudo apt-key add -
```

```bash
# Raspbian 10:
$ echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Raspbian_10/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
$ wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/Raspbian_10/Release.key -O- | sudo apt-key add -
```

```bash
# Ubuntu (18.04, 19.04 and 19.10):
$ . /etc/os-release
$ sudo sh -c "echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/x${NAME}_${VERSION_ID}/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list"
$ wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/x${NAME}_${VERSION_ID}/Release.key -O- | sudo apt-key add -
```

```bash
$ sudo apt-get update -qq
$ sudo apt-get install skopeo
```

Otherwise, read on for building and installing it from source:

To build the `skopeo` binary you need at least Go 1.12.

There are two ways to build skopeo: in a container, or locally without a
container. Choose the one which better matches your needs and environment.

### Building without a container

Building without a container requires a bit more manual work and setup in your
environment, but it is more flexible:

- It should work in more environments (e.g. for native macOS builds)
- It does not require root privileges (after dependencies are installed)
- It is faster, therefore more convenient for developing `skopeo`.

Install the necessary dependencies:

```bash
# Fedora:
$ sudo dnf install gpgme-devel libassuan-devel btrfs-progs-devel device-mapper-devel
```

```bash
# Ubuntu (`libbtrfs-dev` requires Ubuntu 18.10 and above):
$ sudo apt install libgpgme-dev libassuan-dev libbtrfs-dev libdevmapper-dev
```

```bash
# macOS:
$ brew install gpgme
```

```bash
# openSUSE:
$ sudo zypper install libgpgme-devel device-mapper-devel libbtrfs-devel glib2-devel
```

Make sure to clone this repository in your `GOPATH` - otherwise compilation fails.

```bash
$ git clone https://github.com/containers/skopeo $GOPATH/src/github.com/containers/skopeo
$ cd $GOPATH/src/github.com/containers/skopeo
$ make bin/skopeo
# To build a pure-Go static binary (disables devicemapper, btrfs, and gpgme):
$ make bin/skopeo DISABLE_CGO=1
```

### Building in a container

Building in a container is simpler, but more restrictive:

- It requires the `podman` command and the ability to run Linux containers
- The created executable is a Linux executable, and depends on dynamic libraries
  which may only be available only in a container of a similar Linux
  distribution.

```bash
$ make binary # Or (make all) to also build documentation, see below.
```

To build a pure-Go static binary (disables devicemapper, btrfs, and gpgme):

```bash
$ make binary DISABLE_CGO=1
```

### Building documentation

To build the manual you will need go-md2man.

```bash
# Debian:
$ sudo apt-get install go-md2man
```

```
# Fedora:
$ sudo dnf install go-md2man
```

Then

```bash
$ make docs
```

### Installation

Finally, after the binary and documentation is built:

```bash
$ sudo make install
```
