# Installing from packages

## Distribution Packages
`skopeo` may already be packaged in your distribution.

### Fedora

```sh
$ sudo dnf -y install skopeo
### RHEL/CentOS ≥ 8 and CentOS Stream

```sh
sudo dnf -y install skopeo
```

Newer Skopeo releases may be available on the repositories provided by the
Kubic project. Beware, these may not be suitable for production environments.

on CentOS 8:

```sh
sudo dnf -y module disable container-tools
sudo dnf -y install 'dnf-command(copr)'
sudo dnf -y copr enable rhcontainerbot/container-selinux
sudo curl -L -o /etc/yum.repos.d/devel:kubic:libcontainers:stable.repo https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/CentOS_8/devel:kubic:libcontainers:stable.repo
sudo dnf -y install skopeo
```

on CentOS 8 Stream:

```sh
sudo dnf -y module disable container-tools
sudo dnf -y install 'dnf-command(copr)'
sudo dnf -y copr enable rhcontainerbot/container-selinux
sudo curl -L -o /etc/yum.repos.d/devel:kubic:libcontainers:stable.repo https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/CentOS_8_Stream/devel:kubic:libcontainers:stable.repo
sudo dnf -y install skopeo
```

### RHEL/CentOS ≤ 7.x

```sh
$ sudo yum -y install skopeo
```

Newer Skopeo releases may be available on the repositories provided by the
Kubic project. Beware, these may not be suitable for production environments.

```sh
$ sudo curl -L -o /etc/yum.repos.d/devel:kubic:libcontainers:stable.repo https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/CentOS_7/devel:kubic:libcontainers:stable.repo
$ sudo yum -y install skopeo
```

### openSUSE

```sh
sudo zypper install skopeo
```

### Alpine

```sh
sudo apk add skopeo
```

### macOS

```sh
brew install skopeo
```

### Debian ≥ 10 and Ubuntu ≥ 18.04
Debian (10 and newer including Raspbian) and Ubuntu (18.04 and newer): Packages
are available via the [Kubic][0] project repositories:

[0]: https://build.opensuse.org/project/show/devel:kubic:libcontainers:stable

```bash
# Debian Unstable/Sid:
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_Unstable/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
curl -L https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_Unstable/Release.key | sudo apt-key add -
```

```bash
# Debian Testing:
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_Testing/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
curl -L https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_Testing/Release.key | sudo apt-key add -
```

```bash
# Debian 10:
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_10/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
curl -L https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_10/Release.key | sudo apt-key add -
```

```bash
# Raspbian 10:
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Raspbian_10/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
curl -L https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Raspbian_10/Release.key | sudo apt-key add -
```

```bash
# Ubuntu (18.04, 19.04, 19.10 and 20.04):
. /etc/os-release
sudo sh -c "echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/x${NAME}_${VERSION_ID}/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list"
curl -L https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/x${NAME}_${VERSION_ID}/Release.key | sudo apt-key add -
```

```bash
sudo apt-get -y update
sudo apt-get -y install skopeo
```

Otherwise, read on for building and installing it from source:

To build the `skopeo` binary you need at least Go 1.12.

There are two ways to build skopeo: in a container, or locally without a
container. Choose the one which better matches your needs and environment.

## Building from Source

### Building without a container

Building without a container requires a bit more manual work and setup in your
environment, but it is more flexible:

- It should work in more environments (e.g. for native macOS builds)
- It does not require root privileges (after dependencies are installed)
- It is faster, therefore more convenient for developing `skopeo`.

Install the necessary dependencies:

```bash
# Fedora:
sudo dnf install gpgme-devel libassuan-devel btrfs-progs-devel device-mapper-devel
```

```bash
# Ubuntu (`libbtrfs-dev` requires Ubuntu 18.10 and above):
sudo apt install libgpgme-dev libassuan-dev libbtrfs-dev libdevmapper-dev
```

```bash
# macOS:
brew install gpgme
```

```bash
# openSUSE:
sudo zypper install libgpgme-devel device-mapper-devel libbtrfs-devel glib2-devel
```

Make sure to clone this repository in your `GOPATH` - otherwise compilation fails.

```bash
git clone https://github.com/containers/skopeo $GOPATH/src/github.com/containers/skopeo
cd $GOPATH/src/github.com/containers/skopeo && make bin/skopeo
```

### Building in a container

Building in a container is simpler, but more restrictive:

- It requires the `podman` command and the ability to run Linux containers
- The created executable is a Linux executable, and depends on dynamic libraries
  which may only be available only in a container of a similar Linux
  distribution.

```bash
make binary # Or (make all) to also build documentation, see below.
```

### Building documentation

To build the manual you will need go-md2man.

```bash
# Debian:
sudo apt-get install go-md2man
```

```
# Fedora:
sudo dnf install go-md2man
```

Then

```bash
make docs
```

### Installation

Finally, after the binary and documentation is built:

```bash
sudo make install
```
