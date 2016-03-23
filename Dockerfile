FROM fedora

RUN dnf -y update && dnf install -y make git golang golang-github-cpuguy83-go-md2man \
	# registry v1 deps
	xz-devel \
	python-devel \
	python-pip \
	swig \
	redhat-rpm-config \
	openssl-devel \
	patch

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
	&& rm -rf "$GOPATH" \
	&& export DRV1="$(mktemp -d)" \
	&& git clone https://github.com/docker/docker-registry.git "$DRV1" \
	# no need for setuptools since we have a version conflict with fedora
	&& sed -i.bak s/setuptools==5.8//g "$DRV1/requirements/main.txt" \
	&& sed -i.bak s/setuptools==5.8//g "$DRV1/depends/docker-registry-core/requirements/main.txt" \
	&& pip install "$DRV1/depends/docker-registry-core" \
	&& pip install file://"$DRV1#egg=docker-registry[bugsnag,newrelic,cors]" \
	&& patch $(python -c 'import boto; import os; print os.path.dirname(boto.__file__)')/connection.py \
		< "$DRV1/contrib/boto_header_patch.diff" \
	&& dnf -y update && dnf install -y m2crypto

# TODO(runcom): prepopulate registries with images to test skopeo or install docker and push...
# use bats also https://github.com/opencontainers/runc/pull/659/files but need to setup registries as well

# install bats
RUN cd /tmp \
	&& git clone https://github.com/sstephenson/bats.git \
	&& cd bats \
	&& ./install.sh /usr/local

ENV GOPATH /usr/share/gocode:/go
ENV PATH $GOPATH/bin:/usr/share/gocode/bin:$PATH
RUN go get github.com/golang/lint/golint
WORKDIR /go/src/github.com/projectatomic/skopeo
COPY . /go/src/github.com/projectatomic/skopeo

#ENTRYPOINT ["hack/dind"]
