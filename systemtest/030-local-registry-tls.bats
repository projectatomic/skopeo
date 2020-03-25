#!/usr/bin/env bats
#
# Confirm that skopeo will push to and pull from a local
# registry with locally-created TLS certificates.
#
load helpers

function setup() {
    standard_setup

    start_registry --with-cert reg
}

@test "local registry, with cert" {
    # Push to local registry...
    run_skopeo copy --dest-cert-dir=$TESTDIR/client-auth \
               docker://docker.io/library/busybox:latest \
               docker://localhost:5000/busybox:unsigned

    # ...and pull it back out
    run_skopeo copy --src-cert-dir=$TESTDIR/client-auth \
               docker://localhost:5000/busybox:unsigned \
               dir:$TESTDIR/extracted
}

teardown() {
    podman rm -f reg

    standard_teardown
}

# vim: filetype=sh
