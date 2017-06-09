FROM fedora

RUN dnf -y update && dnf install -y make git golang golang-github-cpuguy83-go-md2man \
	# storage deps
	btrfs-progs-devel \
	device-mapper-devel \
	# gpgme bindings deps
	libassuan-devel gpgme-devel \
	gnupg

# Install two versions of the registry. The first is an older version that
# only supports schema1 manifests. The second is a newer version that supports
# both. This allows integration-cli tests to cover push/pull with both schema1
# and schema2 manifests.
ENV REGISTRY_COMMIT_SCHEMA1 ec87e9b6971d831f0eff752ddb54fb64693e51cd
ENV REGISTRY_COMMIT 47a064d4195a9b56133891bbb13620c3ac83a827
RUN set -x \
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
	&& yum install -y which git tar wget hostname util-linux bsdtar socat ethtool device-mapper iptables tree findutils nmap-ncat e2fsprogs xfsprogs lsof docker iproute \
	&& export GOPATH=$(mktemp -d) \
	&& git clone -b v1.5.0-alpha.3 git://github.com/openshift/origin "$GOPATH/src/github.com/openshift/origin" \
	&& (cd "$GOPATH/src/github.com/openshift/origin" && make clean build && make all WHAT=cmd/dockerregistry) \
	&& cp -a "$GOPATH/src/github.com/openshift/origin/_output/local/bin/linux"/*/* /usr/local/bin \
	&& cp "$GOPATH/src/github.com/openshift/origin/images/dockerregistry/config.yml" /atomic-registry-config.yml \
	&& mkdir /registry

ENV GOPATH /usr/share/gocode:/go
ENV PATH $GOPATH/bin:/usr/share/gocode/bin:$PATH
RUN go get github.com/golang/lint/golint
RUN go get github.com/cpuguy83/go-md2man
WORKDIR /go/src/github.com/projectatomic/skopeo
COPY . /go/src/github.com/projectatomic/skopeo

#ENTRYPOINT ["hack/dind"]
