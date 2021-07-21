#!/bin/bash

# This script is intended to be executed by automation or humans
# under a hack/get_ci_vm.sh context.  Use under any other circumstances
# is unlikely to function.

set -e

if [[ -r "/etc/automation_environment" ]]; then
    source /etc/automation_environment
    source $AUTOMATION_LIB_PATH/common_lib.sh
else
    (
    echo "WARNING: It does not appear that containers/automation was installed."
    echo "         Functionality of most of ${BASH_SOURCE[0]} will be negatively"
    echo "         impacted."
    ) > /dev/stderr
fi

OS_RELEASE_ID="$(source /etc/os-release; echo $ID)"
# GCE image-name compatible string representation of distribution _major_ version
OS_RELEASE_VER="$(source /etc/os-release; echo $VERSION_ID | tr -d '.')"
# Combined to ease some usage
OS_REL_VER="${OS_RELEASE_ID}-${OS_RELEASE_VER}"
# This is the magic interpreted by the tests to allow modifying local config/services.
export SKOPEO_CONTAINER_TESTS=1

export "PATH=$PATH:$GOPATH/bin"

_run_setup() {
    local mnt
    local errmsg
    req_env_vars SKOPEO_CI_CONTAINER_FQIN
    if [[ "$OS_RELEASE_ID" == "fedora" ]]; then
        # This is required as part of the standard Fedora VM setup
        growpart /dev/sda 1
        resize2fs /dev/sda1

        # VM's come with the distro. skopeo package pre-installed
        dnf erase -y skopeo
    else
        die "Unknown/unsupported distro. $OS_REL_VER"
    fi
}

_run_vendor() {
    make vendor BUILDTAGS="$BUILDTAGS"
}

_run_build() {
    make bin/skopeo BUILDTAGS="$BUILDTAGS"
}

_run_cross() {
    make local-cross BUILDTAGS="$BUILDTAGS"
}

_run_validate() {
    make validate-local BUILDTAGS="$BUILDTAGS"
}

_run_unit() {
    make test-unit-local BUILDTAGS="$BUILDTAGS"
}

_run_integration() {
    # A slew of compiled binaries are pre-built and distributed
    # within the CI/Dev container image, but we want to run
    # things directly on the host VM.  Fortunately they're all
    # located in the container under /usr/local/bin
    msg "Obtaining test binaries from $SKOPEO_CI_CONTAINER_FQIN"
    podman pull --quiet $SKOPEO_CI_CONTAINER_FQIN
    mnt=$(podman mount $(podman create $SKOPEO_CI_CONTAINER_FQIN))

    # This should never ever ever happen, so double-check to make sure.
    errmsg="Somehow $SKOPEO_CI_CONTAINER_FQIN is not based on $OS_REL_VER."
    if fgrep -qx "ID=$OS_RELEASE_ID" $mnt/etc/os-release; then
        if ! fgrep -qx "VERSION_ID=$OS_RELEASE_VER" $mnt/etc/os-release; then
            die "$errmsg"
        fi
    else
        die "$errmsg"
    fi
    cp -av "$mnt/usr/local/bin/"* "/usr/local/bin/"
    msg "Cleaning up"
    podman umount --latest
    podman rm --latest

    # Ensure we start with a clean-slate
    podman system reset --force
    make test-integration-local BUILDTAGS="$BUILDTAGS"
}

_run_system() {
    # Ensure we start with a clean-slate
    podman system reset --force

    # Executes with containers required for testing.
    make test-system-local BUILDTAGS="$BUILDTAGS"
}

req_env_vars SKOPEO_PATH BUILDTAGS

handler="_run_${1}"
if [ "$(type -t $handler)" != "function" ]; then
    die "Unknown/Unsupported command-line argument '$1'"
fi

msg "************************************************************"
msg "Runner executing $1 on $OS_REL_VER"
msg "************************************************************"

cd "$SKOPEO_PATH"
$handler
