.PHONY: all binary build-container docs docs-in-container build-local clean install install-binary install-completions shell test-integration vendor

export GO15VENDOREXPERIMENT=1

ifeq ($(shell uname),Darwin)
PREFIX ?= ${DESTDIR}/usr/local
DARWIN_BUILD_TAG=containers_image_ostree_stub
# On macOS, (brew install gpgme) installs it within /usr/local, but /usr/local/include is not in the default search path.
# Rather than hard-code this directory, use gpgme-config. Sadly that must be done at the top-level user
# instead of locally in the gpgme subpackage, because cgo supports only pkg-config, not general shell scripts,
# and gpgme does not install a pkg-config file.
# If gpgme is not installed or gpgme-config can’t be found for other reasons, the error is silently ignored
# (and the user will probably find out because the cgo compilation will fail).
GPGME_ENV := CGO_CFLAGS="$(shell gpgme-config --cflags 2>/dev/null)" CGO_LDFLAGS="$(shell gpgme-config --libs 2>/dev/null)"
else
PREFIX ?= ${DESTDIR}/usr
endif

INSTALLDIR=${PREFIX}/bin
MANINSTALLDIR=${PREFIX}/share/man
CONTAINERSSYSCONFIGDIR=${DESTDIR}/etc/containers
REGISTRIESDDIR=${CONTAINERSSYSCONFIGDIR}/registries.d
SIGSTOREDIR=${DESTDIR}/var/lib/atomic/sigstore
BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions
GO_MD2MAN ?= go-md2man
GO ?= go
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)

ifeq ($(DEBUG), 1)
  override GOGCFLAGS += -N -l
endif

ifeq ($(shell go env GOOS), linux)
  GO_DYN_FLAGS="-buildmode=pie"
endif

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
IMAGE := skopeo-dev$(if $(GIT_BRANCH),:$(GIT_BRANCH))
# set env like gobuildtag?
CONTAINER_CMD := ${CONTAINER_RUNTIME} run --rm -i -e TESTFLAGS="$(TESTFLAGS)" #$(CONTAINER_ENVS)
# if this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
ifeq ($(INTERACTIVE), 1)
	CONTAINER_CMD += -t
endif
CONTAINER_RUN := $(CONTAINER_CMD) "$(IMAGE)"

GIT_COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)

MANPAGES_MD = $(wildcard docs/*.md)

BTRFS_BUILD_TAG = $(shell hack/btrfs_tag.sh) $(shell hack/btrfs_installed_tag.sh)
LIBDM_BUILD_TAG = $(shell hack/libdm_tag.sh)
OSTREE_BUILD_TAG = $(shell hack/ostree_tag.sh)
LOCAL_BUILD_TAGS = $(BTRFS_BUILD_TAG) $(LIBDM_BUILD_TAG) $(OSTREE_BUILD_TAG) $(DARWIN_BUILD_TAG)
BUILDTAGS += $(LOCAL_BUILD_TAGS)

ifeq ($(DISABLE_CGO), 1)
	override BUILDTAGS = containers_image_ostree_stub exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp
endif

#   make all DEBUG=1
#     Note: Uses the -N -l go compiler options to disable compiler optimizations
#           and inlining. Using these build options allows you to subsequently
#           use source debugging tools like delve.
all: binary docs-in-container

# Build a container image (skopeobuild) that has everything we need to build.
# Then do the build and the output (skopeo) should appear in current dir
binary: cmd/skopeo
	${CONTAINER_RUNTIME} build ${BUILD_ARGS} -f Dockerfile.build -t skopeobuildimage .
	${CONTAINER_RUNTIME} run --rm --security-opt label=disable -v $$(pwd):/src/github.com/containers/skopeo \
		skopeobuildimage make binary-local $(if $(DEBUG),DEBUG=$(DEBUG)) BUILDTAGS='$(BUILDTAGS)'

binary-static: cmd/skopeo
	${CONTAINER_RUNTIME} build ${BUILD_ARGS} -f Dockerfile.build -t skopeobuildimage .
	${CONTAINER_RUNTIME} run --rm --security-opt label=disable -v $$(pwd):/src/github.com/containers/skopeo \
		skopeobuildimage make binary-local-static $(if $(DEBUG),DEBUG=$(DEBUG)) BUILDTAGS='$(BUILDTAGS)'

# Build w/o using containers
binary-local:
	$(GPGME_ENV) $(GO) build ${GO_DYN_FLAGS} -ldflags "-X main.gitCommit=${GIT_COMMIT}" -gcflags "$(GOGCFLAGS)" -tags "$(BUILDTAGS)" -o skopeo ./cmd/skopeo

binary-local-static:
	$(GPGME_ENV) $(GO) build -ldflags "-extldflags \"-static\" -X main.gitCommit=${GIT_COMMIT}" -gcflags "$(GOGCFLAGS)" -tags "$(BUILDTAGS)" -o skopeo ./cmd/skopeo

build-container:
	${CONTAINER_RUNTIME} build ${BUILD_ARGS} -t "$(IMAGE)" .

docs/%.1: docs/%.1.md
	$(GO_MD2MAN) -in $< -out $@.tmp && touch $@.tmp && mv $@.tmp $@

docs: $(MANPAGES_MD:%.md=%)

docs-in-container:
	${CONTAINER_RUNTIME} build ${BUILD_ARGS} -f Dockerfile.build -t skopeobuildimage .
	${CONTAINER_RUNTIME} run --rm --security-opt label=disable -v $$(pwd):/src/github.com/containers/skopeo \
		skopeobuildimage make docs $(if $(DEBUG),DEBUG=$(DEBUG)) BUILDTAGS='$(BUILDTAGS)'

clean:
	rm -f skopeo docs/*.1

install: install-binary install-docs install-completions
	install -d -m 755 ${SIGSTOREDIR}
	install -d -m 755 ${CONTAINERSSYSCONFIGDIR}
	install -m 644 default-policy.json ${CONTAINERSSYSCONFIGDIR}/policy.json
	install -d -m 755 ${REGISTRIESDDIR}
	install -m 644 default.yaml ${REGISTRIESDDIR}/default.yaml

install-binary: ./skopeo
	install -d -m 755 ${INSTALLDIR}
	install -m 755 skopeo ${INSTALLDIR}/skopeo

install-docs: docs/skopeo.1
	install -d -m 755 ${MANINSTALLDIR}/man1
	install -m 644 docs/skopeo.1 ${MANINSTALLDIR}/man1/skopeo.1

install-completions:
	install -m 755 -d ${BASHINSTALLDIR}
	install -m 644 completions/bash/skopeo ${BASHINSTALLDIR}/skopeo

shell: build-container
	$(CONTAINER_RUN) bash

check: validate test-unit test-integration

# The tests can run out of entropy and block in containers, so replace /dev/random.
test-integration: build-container
	$(CONTAINER_RUN) bash -c 'rm -f /dev/random; ln -sf /dev/urandom /dev/random; SKOPEO_CONTAINER_TESTS=1 BUILDTAGS="$(BUILDTAGS)" hack/make.sh test-integration'

test-unit: build-container
	# Just call (make test unit-local) here instead of worrying about environment differences, e.g. GO15VENDOREXPERIMENT.
	$(CONTAINER_RUN) make test-unit-local BUILDTAGS='$(BUILDTAGS)'

validate: build-container
	$(CONTAINER_RUN) hack/make.sh validate-git-marks validate-gofmt validate-lint validate-vet

# This target is only intended for development, e.g. executing it from an IDE. Use (make test) for CI or pre-release testing.
test-all-local: validate-local test-unit-local

validate-local:
	hack/make.sh validate-git-marks validate-gofmt validate-lint validate-vet

test-unit-local:
	$(GPGME_ENV) $(GO) test -tags "$(BUILDTAGS)" $$($(GO) list -tags "$(BUILDTAGS)" -e ./... | grep -v '^github\.com/containers/skopeo/\(integration\|vendor/.*\)$$')

vendor: vendor.conf
	vndr -whitelist '^github.com/containers/image/docs/.*'
