.PHONY: all binary build-container docs docs-in-container build-local clean install install-binary install-completions shell test-integration .install.vndr vendor vendor-in-container

export GOPROXY=https://proxy.golang.org

ifeq ($(shell uname),Darwin)
PREFIX ?= ${DESTDIR}/usr/local
DARWIN_BUILD_TAG=
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
SIGSTOREDIR=${DESTDIR}/var/lib/containers/sigstore
BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions

GO ?= go
GOBIN := $(shell $(GO) env GOBIN)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

ifeq ($(GOBIN),)
GOBIN := $(GOPATH)/bin
endif

CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)
GOMD2MAN ?= $(shell command -v go-md2man || echo '$(GOBIN)/go-md2man')

# Go module support: set `-mod=vendor` to use the vendored sources.
# See also hack/make.sh.
ifeq ($(shell go help mod >/dev/null 2>&1 && echo true), true)
  GO:=GO111MODULE=on $(GO)
  MOD_VENDOR=-mod=vendor
endif

ifeq ($(DEBUG), 1)
  override GOGCFLAGS += -N -l
endif

ifeq ($(GOOS), linux)
  ifneq ($(GOARCH),$(filter $(GOARCH),mips mipsle mips64 mips64le ppc64 riscv64))
    GO_DYN_FLAGS="-buildmode=pie"
  endif
endif

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
# Default base container image on which to run tests
DEFAULT_BASE := fedora
# Base image for most commands, overridable by command-line arguments
BASE_IMAGE := fedora
# Base images (one or more) used for (make check), overridable by command-line arguments
CHECK_BASE_IMAGES := $(BASE_IMAGE)
# Name for our containers: skopeo-dev[-base-image][:branch]
define image_name
skopeo-dev$(if $(filter-out $(DEFAULT_BASE),$1),-$1)$(if $(GIT_BRANCH),:$(GIT_BRANCH))
endef
# set env like gobuildtag?
CONTAINER_CMD := ${CONTAINER_RUNTIME} run --rm -i -e TESTFLAGS="$(TESTFLAGS)" #$(CONTAINER_ENVS)
# if this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
ifeq ($(INTERACTIVE), 1)
	CONTAINER_CMD += -t
endif

GIT_COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)

EXTRA_LDFLAGS ?=
SKOPEO_LDFLAGS := -ldflags '-X main.gitCommit=${GIT_COMMIT} $(EXTRA_LDFLAGS)'

MANPAGES_MD = $(wildcard docs/*.md)
MANPAGES ?= $(MANPAGES_MD:%.md=%)

BTRFS_BUILD_TAG = $(shell hack/btrfs_tag.sh) $(shell hack/btrfs_installed_tag.sh)
LIBDM_BUILD_TAG = $(shell hack/libdm_tag.sh)
LOCAL_BUILD_TAGS = $(BTRFS_BUILD_TAG) $(LIBDM_BUILD_TAG) $(DARWIN_BUILD_TAG)
BUILDTAGS += $(LOCAL_BUILD_TAGS)

ifeq ($(DISABLE_CGO), 1)
	override BUILDTAGS = exclude_graphdriver_devicemapper exclude_graphdriver_btrfs containers_image_openpgp
endif

#   make all DEBUG=1
#     Note: Uses the -N -l go compiler options to disable compiler optimizations
#           and inlining. Using these build options allows you to subsequently
#           use source debugging tools like delve.
all: bin/skopeo docs

help:
	@echo "Usage: make <target>"
	@echo
	@echo "Defaults to building bin/skopeo and docs"
	@echo
	@echo " * 'install' - Install binaries and documents to system locations"
	@echo " * 'binary' - Build skopeo with a container"
	@echo " * 'static' - Build statically linked binary"
	@echo " * 'bin/skopeo' - Build skopeo locally"
	@echo " * 'test-unit' - Execute unit tests"
	@echo " * 'test-integration' - Execute integration tests"
	@echo " * 'validate' - Verify whether there is no conflict and all Go source files have been formatted, linted and vetted"
	@echo " * 'check' - Including above validate, test-integration and test-unit"
	@echo " * 'shell' - Run the built image and attach to a shell"
	@echo " * 'clean' - Clean artifacts"

# Build a container image (skopeobuild) that has everything we need to build.
# Then do the build and the output (skopeo) should appear in current dir
binary: cmd/skopeo
	${CONTAINER_RUNTIME} build ${BUILD_ARGS} -f Dockerfile.build -t skopeobuildimage .
	${CONTAINER_RUNTIME} run --rm --security-opt label=disable -v $$(pwd):/src/github.com/containers/skopeo \
		skopeobuildimage make bin/skopeo $(if $(DEBUG),DEBUG=$(DEBUG)) BUILDTAGS='$(BUILDTAGS)'

# Update nix/nixpkgs.json its latest stable commit
.PHONY: nixpkgs
nixpkgs:
	@nix run -f channel:nixos-20.03 nix-prefetch-git -c nix-prefetch-git \
		--no-deepClone https://github.com/nixos/nixpkgs > nix/nixpkgs.json

# Build statically linked binary
.PHONY: static
static:
	@nix build -f nix/
	mkdir -p ./bin
	cp -rfp ./result/bin/* ./bin/

# Build w/o using containers
.PHONY: bin/skopeo
bin/skopeo:
	$(GPGME_ENV) $(GO) build $(MOD_VENDOR) ${GO_DYN_FLAGS} ${SKOPEO_LDFLAGS} -gcflags "$(GOGCFLAGS)" -tags "$(BUILDTAGS)" -o $@ ./cmd/skopeo
bin/skopeo.%:
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build $(MOD_VENDOR) ${SKOPEO_LDFLAGS} -tags "containers_image_openpgp $(BUILDTAGS)" -o $@ ./cmd/skopeo
local-cross: bin/skopeo.darwin.amd64 bin/skopeo.linux.arm bin/skopeo.linux.arm64 bin/skopeo.windows.386.exe bin/skopeo.windows.amd64.exe

build-container: build-container-$(BASE_IMAGE)
build-container-%:
	base=$*; if [ "$$base" = centos ]; then base=centos:7; fi; \
		dockerfile=$$(mktemp -p .); sed "s/^FROM fedora/FROM $$base/" Dockerfile > "$$dockerfile"; \
	${CONTAINER_RUNTIME} build ${BUILD_ARGS} -f $$dockerfile -t "$(call image_name,$*)" . ; exit=$$?; \
	rm "$$dockerfile"; exit $$exit

$(MANPAGES): %: %.md
	sed -e 's/\((skopeo.*\.md)\)//' -e 's/\[\(skopeo.*\)\]/\1/' $<  | $(GOMD2MAN) -in /dev/stdin -out $@

docs: $(MANPAGES)

docs-in-container:
	${CONTAINER_RUNTIME} build ${BUILD_ARGS} -f Dockerfile.build -t skopeobuildimage .
	${CONTAINER_RUNTIME} run --rm --security-opt label=disable -v $$(pwd):/src/github.com/containers/skopeo \
		skopeobuildimage make docs $(if $(DEBUG),DEBUG=$(DEBUG)) BUILDTAGS='$(BUILDTAGS)'

clean:
	rm -rf bin docs/*.1

install: install-binary install-docs install-completions
	install -d -m 755 ${SIGSTOREDIR}
	install -d -m 755 ${CONTAINERSSYSCONFIGDIR}
	install -m 644 default-policy.json ${CONTAINERSSYSCONFIGDIR}/policy.json
	install -d -m 755 ${REGISTRIESDDIR}
	install -m 644 default.yaml ${REGISTRIESDDIR}/default.yaml

install-binary: bin/skopeo
	install -d -m 755 ${INSTALLDIR}
	install -m 755 bin/skopeo ${INSTALLDIR}/skopeo

install-docs: docs
	install -d -m 755 ${MANINSTALLDIR}/man1
	install -m 644 docs/*.1 ${MANINSTALLDIR}/man1/

install-completions:
	install -m 755 -d ${BASHINSTALLDIR}
	install -m 644 completions/bash/skopeo ${BASHINSTALLDIR}/skopeo

shell: shell-$(BASE_IMAGE)
shell-%: build-container-%
	$(CONTAINER_CMD) "$(call image_name,$*)" bash

check: $(foreach image,$(CHECK_BASE_IMAGES),validate-$(image) test-unit-$(image) test-integration-$(image) test-system-$(image))

test-integration: test-integration-$(BASE_IMAGE)
# The tests can run out of entropy and block in containers, so replace /dev/random.
test-integration-%: build-container-%
	$(CONTAINER_CMD) "$(call image_name,$*)" bash -c 'rm -f /dev/random; ln -sf /dev/urandom /dev/random; SKOPEO_CONTAINER_TESTS=1 BUILDTAGS="$(BUILDTAGS)" hack/make.sh test-integration'

test-system: test-system-$(BASE_IMAGE)
# complicated set of options needed to run podman-in-podman
test-system-%: build-container-%
	DTEMP=$(shell mktemp -d --tmpdir=/var/tmp podman-tmp.XXXXXX); \
	$(CONTAINER_CMD) --privileged \
	    -v $$DTEMP:/var/lib/containers:Z -v /run/systemd/journal/socket:/run/systemd/journal/socket \
            "$(call image_name,$*)" \
            bash -c 'BUILDTAGS="$(BUILDTAGS)" hack/make.sh test-system'; \
	rc=$$?; \
	$(RM) -rf $$DTEMP; \
	exit $$rc

test-unit: test-unit-$(BASE_IMAGE)
# Just call (make test unit-local) here instead of worrying about environment differences
test-unit-%: build-container-%
	$(CONTAINER_CMD) "$(call image_name,$*)" make test-unit-local BUILDTAGS='$(BUILDTAGS)'

validate: validate-$(BASE_IMAGE)
validate-%: build-container-%
	$(CONTAINER_CMD) "$(call image_name,$*)" hack/make.sh validate-git-marks validate-gofmt validate-lint validate-vet

# This target is only intended for development, e.g. executing it from an IDE. Use (make test) for CI or pre-release testing.
test-all-local: validate-local test-unit-local

validate-local:
	hack/make.sh validate-git-marks validate-gofmt validate-lint validate-vet

test-unit-local:
	$(GPGME_ENV) $(GO) test $(MOD_VENDOR) -tags "$(BUILDTAGS)" $$($(GO) list $(MOD_VENDOR) -tags "$(BUILDTAGS)" -e ./... | grep -v '^github\.com/containers/skopeo/\(integration\|vendor/.*\)$$')

vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

vendor-in-container:
	podman run --privileged --rm --env HOME=/root -v `pwd`:/src -w /src docker.io/library/golang:1.13 make vendor
