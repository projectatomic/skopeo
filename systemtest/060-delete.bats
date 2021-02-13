#!/usr/bin/env bats
#
# Copy tests
#

load helpers

function setup() {
    standard_setup

    start_registry --enable-delete=true reg
}

# delete image from registry
@test "delete: remove image from registry" {
    local remote_image=docker://quay.io/libpod/busybox:latest
    local localimg=docker://localhost:5000/busybox:unsigned
    local output=

    run_skopeo copy --dest-tls-verify=false $remote_image $localimg
    output=$(run_skopeo inspect --tls-verify=false --raw $localimg)
    echo $output | grep "vnd.docker.distribution.manifest.v2+json"

    run_skopeo delete --tls-verify=false $localimg

    # make sure image is removed from registry
    expected_rc=1
    run_skopeo $expected_rc inspect --tls-verify=false $localimg
}

teardown() {
    podman rm -f reg

    standard_teardown
}

# vim: filetype=sh
