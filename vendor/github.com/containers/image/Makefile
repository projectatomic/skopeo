.PHONY: all deps test validate lint

# Which github repostiory and branch to use for testing with skopeo
SKOPEO_REPO = projectatomic/skopeo
SKOPEO_BRANCH = master
# Set SUDO=sudo to run container integration tests using sudo.
SUDO =
BUILDTAGS   = btrfs_noversion libdm_no_deferred_remove
BUILDFLAGS := -tags "$(BUILDTAGS)"

all: deps .gitvalidation test validate

deps:
	go get -t $(BUILDFLAGS) ./...
	go get -u $(BUILDFLAGS) github.com/golang/lint/golint
	go get $(BUILDFLAGS) github.com/vbatts/git-validation

test:
	@go test $(BUILDFLAGS) -cover ./...

# This is not run as part of (make all), but Travis CI does run this.
# Demonstarting a working version of skopeo (possibly with modified SKOPEO_REPO/SKOPEO_BRANCH, e.g.
#    make test-skopeo SKOPEO_REPO=runcom/skopeo-1 SKOPEO_BRANCH=oci-3 SUDO=sudo
# ) is a requirement before merging; note that Travis will only test
# the master branch of the upstream repo.
test-skopeo:
	@echo === Testing skopeo build
	@export GOPATH=$$(mktemp -d) && \
		skopeo_path=$${GOPATH}/src/github.com/projectatomic/skopeo && \
		vendor_path=$${skopeo_path}/vendor/github.com/containers/image && \
		git clone -b $(SKOPEO_BRANCH) https://github.com/$(SKOPEO_REPO) $${skopeo_path} && \
		rm -rf $${vendor_path} && cp -r . $${vendor_path} && \
		cd $${skopeo_path} && \
		make BUILDTAGS="$(BUILDTAGS)" binary-local test-all-local && \
		$(SUDO) make check && \
		rm -rf $${skopeo_path}

validate: lint
	@go vet ./...
	@test -z "$$(gofmt -s -l . | tee /dev/stderr)"

lint:
	@out="$$(golint ./...)"; \
	if [ -n "$$(golint ./...)" ]; then \
		echo "$$out"; \
		exit 1; \
	fi

.PHONY: .gitvalidation

EPOCH_TEST_COMMIT ?= e68e0e1110e64f906f9b482e548f17d73e02e6b1

# When this is running in travis, it will only check the travis commit range
.gitvalidation:
	@which git-validation > /dev/null 2>/dev/null || (echo "ERROR: git-validation not found. Consider 'make deps' target" && false)
ifeq ($(TRAVIS),true)
	@git-validation -q -run DCO,short-subject,dangling-whitespace
else
	@git-validation -q -run DCO,short-subject,dangling-whitespace -range $(EPOCH_TEST_COMMIT)..HEAD
endif
