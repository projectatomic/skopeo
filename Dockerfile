FROM fedora

RUN dnf -y update && dnf install -y make git golang golang-github-cpuguy83-go-md2man \
	libassuan-devel gpgme-devel \
	gnupg \
	xz-devel \
	python-devel \
	python-pip \
	swig \
	redhat-rpm-config \
	openssl-devel \
	patch \
	jq

# Install three versions of the registry. The first is an older version that
# only supports schema1 manifests. The second is a newer version that supports
# both. This allows integration-cli tests to cover push/pull with both schema1
# and schema2 manifests. Install registry v1 also.
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
	&& git clone -b v1.3.0-alpha.3 git://github.com/openshift/origin "$GOPATH/src/github.com/openshift/origin" \
	&& (cd "$GOPATH/src/github.com/openshift/origin" && make clean build && make all WHAT=cmd/dockerregistry) \
	&& cp -a "$GOPATH/src/github.com/openshift/origin/_output/local/bin/linux"/*/* /usr/local/bin \
	&& cp "$GOPATH/src/github.com/openshift/origin/images/dockerregistry/config.yml" /atomic-registry-config.yml \
	&& mkdir /registry


ENV GOPATH /usr/share/gocode:/go
ENV PATH $GOPATH/bin:/usr/share/gocode/bin:$PATH
RUN go get github.com/golang/lint/golint
WORKDIR /go/src/github.com/projectatomic/skopeo
COPY . /go/src/github.com/projectatomic/skopeo

# Get useful and necessary Hub images so we can "docker load" locally instead of pulling
RUN ./hack/download-frozen-image-v2.sh /docker-frozen-images \
	buildpack-deps:jessie@sha256:25785f89240fbcdd8a74bdaf30dd5599a9523882c6dfc567f2e9ef7cf6f79db6 \
	busybox:latest@sha256:e4f93f6ed15a0cdd342f5aae387886fba0ab98af0a102da6276eaf24d6e6ade0 \
	debian:jessie@sha256:f968f10b4b523737e253a97eac59b0d1420b5c19b69928d35801a6373ffe330e \
	hello-world:latest@sha256:8be990ef2aeb16dbcb9271ddfe2610fa6658d13f6dfb8bc72074cc1ca36966a7

#ENTRYPOINT ["hack/dind"]
