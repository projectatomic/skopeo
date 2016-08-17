.PHONY: all binary build-container build-local clean install install-binary shell test-integration

export GO15VENDOREXPERIMENT=1

PREFIX ?= ${DESTDIR}/usr
INSTALLDIR=${PREFIX}/bin
MANINSTALLDIR=${PREFIX}/share/man
# TODO(runcom)
#BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions
GO_MD2MAN ?= /usr/bin/go-md2man

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
# Default Docker base image on which to run tests
DEFAULT_BASE := fedora
# Base image for most commands, overridable by command-line arguments
BASE_IMAGE := fedora
# Base images (one or more) used for (make check), overridable by command-line arguments
CHECK_BASE_IMAGES := $(BASE_IMAGE)
# Name for our containers: skopeo-dev[-base-image][:branch]
define docker_image_name
skopeo-dev$(if $(filter-out $(DEFAULT_BASE),$1),-$1)$(if $(GIT_BRANCH),:$(GIT_BRANCH))
endef
# set env like gobuildtag?
DOCKER_RUN := docker run --rm -i #$(DOCKER_ENVS)
# if this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
ifeq ($(INTERACTIVE), 1)
	DOCKER_RUN += -t
endif

GIT_COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)

MANPAGES_MD = $(wildcard docs/*.md)

all: binary docs

# Build a docker image (skopeobuild) that has everything we need to build.
# Then do the build and the output (skopeo) should appear in current dir
binary: cmd/skopeo
	docker build ${DOCKER_BUILD_ARGS} -f Dockerfile.build -t skopeobuildimage .
	docker run --rm -v ${PWD}:/src/github.com/projectatomic/skopeo \
		skopeobuildimage make binary-local

# Build w/o using Docker containers
binary-local:
	go build -ldflags "-X main.gitCommit=${GIT_COMMIT}" -o skopeo ./cmd/skopeo

build-container: build-container-$(BASE_IMAGE)
build-container-%:
	dockerfile=$$(mktemp -p .); sed 's/^FROM fedora/FROM $*/' Dockerfile > "$$dockerfile"; \
	docker build ${DOCKER_BUILD_ARGS} -f $$dockerfile -t "$(call docker_image_name,$*)" . ; exit=$$?; \
	rm "$$dockerfile"; exit $$exit

docs/%.1: docs/%.1.md
	$(GO_MD2MAN) -in $< -out $@.tmp && touch $@.tmp && mv $@.tmp $@

.PHONY: docs
docs: $(MANPAGES_MD:%.md=%)

clean:
	rm -f skopeo docs/*.1

install: install-binary install-docs
	# TODO(runcom)
	#install -m 644 completion/bash/skopeo ${BASHINSTALLDIR}/

install-binary: ./skopeo
	install -d -m 0755 ${INSTALLDIR}
	install -m 755 skopeo ${INSTALLDIR}

install-docs: docs/skopeo.1
	install -d -m 0755 ${MANINSTALLDIR}/man1
	install -m 644 docs/skopeo.1 ${MANINSTALLDIR}/man1/

shell: shell-$(BASE_IMAGE)
shell-%: build-container-%
	$(DOCKER_RUN) "$(call docker_image_name,$*)" bash

check: $(foreach image,$(CHECK_BASE_IMAGES),validate-$(image) test-unit-$(image) test-integration-$(image))

test-integration: test-integration-$(BASE_IMAGE)
# The tests can run out of entropy and block in containers, so replace /dev/random.
test-integration-%: build-container-%
	$(DOCKER_RUN) "$(call docker_image_name,$*)" bash -c 'rm -f /dev/random; ln -sf /dev/urandom /dev/random; SKOPEO_CONTAINER_TESTS=1 hack/make.sh test-integration'

test-unit: test-unit-$(BASE_IMAGE)
# Just call (make test unit-local) here instead of worrying about environment differences, e.g. GO15VENDOREXPERIMENT.
test-unit-%: build-container-%
	$(DOCKER_RUN) "$(call docker_image_name,$*)" make test-unit-local

validate: validate-$(BASE_IMAGE)
# SKIP_GOLINT_IF_MISSING can be used to handle CentOS with Go 1.4; this should not be a part of regular development workflow,
# and will be dropped as soon as Go is upgraded.
validate-%: build-container-%
	lint=validate-lint; if [ -n "$(SKIP_GOLINT_IF_MISSING)" ] && ! type golint &>/dev/null ; then lint= ; fi; \
	$(DOCKER_RUN) "$(call docker_image_name,$*)" hack/make.sh validate-git-marks validate-gofmt $$lint validate-vet

# This target is only intended for development, e.g. executing it from an IDE. Use (make test) for CI or pre-release testing.
test-all-local: validate-local test-unit-local

validate-local:
	hack/make.sh validate-git-marks validate-gofmt validate-lint validate-vet

test-unit-local:
	go test $$(go list -e ./... | grep -v '^github\.com/projectatomic/skopeo/\(integration\|vendor/.*\)$$')
