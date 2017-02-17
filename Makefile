.PHONY: all binary build-container build-local clean install install-binary install-completions shell test-integration

export GO15VENDOREXPERIMENT=1

PREFIX ?= ${DESTDIR}/usr
INSTALLDIR=${PREFIX}/bin
MANINSTALLDIR=${PREFIX}/share/man
CONTAINERSSYSCONFIGDIR=${DESTDIR}/etc/containers
REGISTRIESDDIR=${CONTAINERSSYSCONFIGDIR}/registries.d
SIGSTOREDIR=${DESTDIR}/var/lib/atomic/sigstore
BASHINSTALLDIR=${PREFIX}/share/bash-completion/completions
GO_MD2MAN ?= go-md2man

ifeq ($(DEBUG), 1)
  override GOGCFLAGS += -N -l
endif

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
DOCKER_IMAGE := skopeo-dev$(if $(GIT_BRANCH),:$(GIT_BRANCH))
# set env like gobuildtag?
DOCKER_FLAGS := docker run --rm -i #$(DOCKER_ENVS)
# if this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
ifeq ($(INTERACTIVE), 1)
	DOCKER_FLAGS += -t
endif
DOCKER_RUN_DOCKER := $(DOCKER_FLAGS) "$(DOCKER_IMAGE)"

GIT_COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)

MANPAGES_MD = $(wildcard docs/*.md)

BTRFS_BUILD_TAG = $(shell hack/btrfs_tag.sh)
LIBDM_BUILD_TAG = $(shell hack/libdm_tag.sh)
LOCAL_BUILD_TAGS = $(BTRFS_BUILD_TAG) $(LIBDM_BUILD_TAG)
BUILDTAGS += $(LOCAL_BUILD_TAGS)

#   make all DEBUG=1
#     Note: Uses the -N -l go compiler options to disable compiler optimizations
#           and inlining. Using these build options allows you to subsequently
#           use source debugging tools like delve.
all: binary docs

# Build a docker image (skopeobuild) that has everything we need to build.
# Then do the build and the output (skopeo) should appear in current dir
binary: cmd/skopeo
	docker build ${DOCKER_BUILD_ARGS} -f Dockerfile.build -t skopeobuildimage .
	docker run --rm --security-opt label:disable -v $$(pwd):/src/github.com/projectatomic/skopeo \
		skopeobuildimage make binary-local $(if $(DEBUG),DEBUG=$(DEBUG))

binary-static: cmd/skopeo
	docker build ${DOCKER_BUILD_ARGS} -f Dockerfile.build -t skopeobuildimage .
	docker run --rm --security-opt label:disable -v $$(pwd):/src/github.com/projectatomic/skopeo \
		skopeobuildimage make binary-local-static $(if $(DEBUG),DEBUG=$(DEBUG))

# Build w/o using Docker containers
binary-local:
	go build -ldflags "-X main.gitCommit=${GIT_COMMIT}" -gcflags "$(GOGCFLAGS)" -tags "$(BUILDTAGS)" -o skopeo ./cmd/skopeo

binary-local-static:
	go build -ldflags "-extldflags \"-static\" -X main.gitCommit=${GIT_COMMIT}" -gcflags "$(GOGCFLAGS)" -tags "$(BUILDTAGS)" -o skopeo ./cmd/skopeo

build-container:
	docker build ${DOCKER_BUILD_ARGS} -t "$(DOCKER_IMAGE)" .

docs/%.1: docs/%.1.md
	$(GO_MD2MAN) -in $< -out $@.tmp && touch $@.tmp && mv $@.tmp $@

.PHONY: docs
docs: $(MANPAGES_MD:%.md=%)

clean:
	rm -f skopeo docs/*.1

install: install-binary install-docs install-completions
	install -d -m 755 ${SIGSTOREDIR}
	install -D -m 644 default-policy.json ${CONTAINERSSYSCONFIGDIR}/policy.json
	install -D -m 644 default.yaml ${REGISTRIESDDIR}/default.yaml

install-binary: ./skopeo
	install -D -m 755 skopeo ${INSTALLDIR}/skopeo

install-docs: docs/skopeo.1
	install -D -m 644 docs/skopeo.1 ${MANINSTALLDIR}/man1/skopeo.1

install-completions:
	install -m 644 -D completions/bash/skopeo ${BASHINSTALLDIR}/skopeo

shell: build-container
	$(DOCKER_RUN_DOCKER) bash

check: validate test-unit test-integration

# The tests can run out of entropy and block in containers, so replace /dev/random.
test-integration: build-container
	$(DOCKER_RUN_DOCKER) bash -c 'rm -f /dev/random; ln -sf /dev/urandom /dev/random; SKOPEO_CONTAINER_TESTS=1 hack/make.sh test-integration'

test-unit: build-container
	# Just call (make test unit-local) here instead of worrying about environment differences, e.g. GO15VENDOREXPERIMENT.
	$(DOCKER_RUN_DOCKER) make test-unit-local

validate: build-container
	$(DOCKER_RUN_DOCKER) hack/make.sh validate-git-marks validate-gofmt validate-lint validate-vet

# This target is only intended for development, e.g. executing it from an IDE. Use (make test) for CI or pre-release testing.
test-all-local: validate-local test-unit-local

validate-local:
	hack/make.sh validate-git-marks validate-gofmt validate-lint validate-vet

test-unit-local:
	# First run our own tests.
	go test -tags "$(BUILDTAGS)" $$(go list -tags "$(BUILDTAGS)" -e ./... | grep -v '^github\.com/projectatomic/skopeo/\(integration\|vendor/.*\)$$')
	# containers/image is a special case -- we _want_ to run its unit tests
	# here to make sure our vendored packages work with it (and
	# containers/image doesn't vendor anything).
	go test -tags "$(BUILDTAGS)" $$(go list -tags "$(BUILDTAGS)" -e ./... | grep '^github\.com/projectatomic/skopeo/vendor/github\.com/containers/image/.*$$')
