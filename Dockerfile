FROM fedora

RUN dnf -y update && dnf install -y make git golang golang-github-cpuguy83-md2man \
	# storage deps
	btrfs-progs-devel \
	device-mapper-devel \
	# gpgme bindings deps
	libassuan-devel gpgme-devel \
	gnupg \
	# htpasswd for system tests
	httpd-tools \
	# OpenShift deps
	which tar wget hostname util-linux bsdtar socat ethtool device-mapper iptables tree findutils nmap-ncat e2fsprogs xfsprogs lsof docker iproute \
        bats jq podman runc \
	golint \
	openssl \
	&& dnf clean all

# Install two versions of the registry. The first is an older version that
# only supports schema1 manifests. The second is a newer version that supports
# both. This allows integration-cli tests to cover push/pull with both schema1
# and schema2 manifests.
RUN set -x \
	&& REGISTRY_COMMIT_SCHEMA1=ec87e9b6971d831f0eff752ddb54fb64693e51cd \
	&& REGISTRY_COMMIT=47a064d4195a9b56133891bbb13620c3ac83a827 \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/docker/distribution.git "$GOPATH/src/github.com/docker/distribution" \
	&& (cd "$GOPATH/src/github.com/docker/distribution" && git checkout -q "$REGISTRY_COMMIT") \
	&& GOPATH="$GOPATH/src/github.com/docker/distribution/Godeps/_workspace:$GOPATH" \
		go build -o /usr/local/bin/registry-v2 github.com/docker/distribution/cmd/registry \
	&& (cd "$GOPATH/src/github.com/docker/distribution" && git checkout -q "$REGISTRY_COMMIT_SCHEMA1") \
	&& GOPATH="$GOPATH/src/github.com/docker/distribution/Godeps/_workspace:$GOPATH" \
		go build -o /usr/local/bin/registry-v2-schema1 github.com/docker/distribution/cmd/registry \
	&& rm -rf "$GOPATH"

RUN set -x \
	&& export GOPATH=$(mktemp -d) \
	&& git clone --depth 1 -b v1.5.0-alpha.3 git://github.com/openshift/origin "$GOPATH/src/github.com/openshift/origin" \
	# The sed edits out a "go < 1.5" check which works incorrectly with go ≥ 1.10. \
	&& sed -i -e 's/\[\[ "\${go_version\[2]}" < "go1.5" ]]/false/' "$GOPATH/src/github.com/openshift/origin/hack/common.sh" \
	&& (cd "$GOPATH/src/github.com/openshift/origin" && make clean build && make all WHAT=cmd/dockerregistry) \
	&& cp -a "$GOPATH/src/github.com/openshift/origin/_output/local/bin/linux"/*/* /usr/local/bin \
	&& cp "$GOPATH/src/github.com/openshift/origin/images/dockerregistry/config.yml" /atomic-registry-config.yml \
	&& rm -rf "$GOPATH" \
	&& mkdir /registry

ENV GOPATH /usr/share/gocode:/go
ENV PATH $GOPATH/bin:/usr/share/gocode/bin:$PATH
ENV container_magic 85531765-346b-4316-bdb8-358e4cca9e5d
RUN go version
WORKDIR /go/src/github.com/containers/skopeo
COPY . /go/src/github.com/containers/skopeo

#ENTRYPOINT ["hack/dind"]
