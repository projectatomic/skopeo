FROM fedora

RUN if (. /etc/os-release ; [ "$ID" != centos ]); then dnf=dnf; else dnf=yum; $dnf install -y epel-release; fi \
	&& $dnf -y update && $dnf install -y make git golang golang-github-cpuguy83-go-md2man \
	# gpgme bindings deps
	libassuan-devel gpgme-devel \
	gnupg \
	# registry v1 deps
	xz-devel \
	python-devel \
	python-pip \
	swig \
	redhat-rpm-config \
	openssl-devel \
	m2crypto \
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
	# drop M2Crypto dependency, that one does not build with just pip on CentOS, and we have a newer version in the distribution anyway.
	&& sed -i.nom2 /M2Crypto==0.22.3/d "$DRV1/requirements/main.txt" \
	# no need for setuptools since we have a version conflict with fedora
	&& sed -i.bak s/setuptools==5.8//g "$DRV1/requirements/main.txt" \
	&& sed -i.bak s/setuptools==5.8//g "$DRV1/depends/docker-registry-core/requirements/main.txt" \
	&& pip install "$DRV1/depends/docker-registry-core" \
	&& pip install file://"$DRV1#egg=docker-registry[bugsnag,newrelic,cors]" \
	&& patch $(python -c 'import boto; import os; print os.path.dirname(boto.__file__)')/connection.py \
		< "$DRV1/contrib/boto_header_patch.diff"

RUN set -x \
	&& yum install -y which git tar wget hostname util-linux bsdtar socat ethtool device-mapper iptables tree findutils nmap-ncat e2fsprogs xfsprogs lsof docker iproute \
	&& export GOPATH=$(mktemp -d) \
	# && git clone git://github.com/openshift/origin "$GOPATH/src/github.com/openshift/origin" \
	# && git clone -b image-signatures-rest git://github.com/miminar/origin "$GOPATH/src/github.com/openshift/origin" \
	&& git clone -b image-signatures-rest-backup git://github.com/mtrmac/origin "$GOPATH/src/github.com/openshift/origin" \
	&& (cd "$GOPATH/src/github.com/openshift/origin" && make clean build && make all WHAT=cmd/dockerregistry) \
	&& cp -a "$GOPATH/src/github.com/openshift/origin/_output/local/bin/linux"/*/* /usr/local/bin \
	&& cp "$GOPATH/src/github.com/openshift/origin/images/dockerregistry/config.yml" /atomic-registry-config.yml \
	&& mkdir /registry

ENV GOPATH /usr/share/gocode:/go:/go/src/github.com/projectatomic/skopeo/vendor
ENV PATH $GOPATH/bin:/usr/share/gocode/bin:$PATH
# golint does not build against Go 1.4, and vet is not included in the package.
RUN if go version | grep -qF 'go1.4'; then yum install -y golang-vet ; else go get github.com/golang/lint/golint ; fi
WORKDIR /go/src/github.com/projectatomic/skopeo
COPY . /go/src/github.com/projectatomic/skopeo
# Go 1.4 does not support GO15VENDOREXPERIMENT; emulate by using GOPATH pointing to the vendor directory, and making vendor/src == vendor
RUN { ! go version | grep -qF 'go1.4' ; } || { rm -rf vendor/src; ln -s . vendor/src ; }

#ENTRYPOINT ["hack/dind"]
