#!/usr/bin/env bats
#
# Confirm that skopeo will push to and pull from a local
# registry with locally-created TLS certificates.
#
load helpers

function setup() {
    standard_setup

    start_registry --with-cert --enable-delete=true reg
}

@test "local registry, with cert" {
    # Push to local registry...
    run_skopeo copy --dest-cert-dir=$TESTDIR/client-auth \
               docker://quay.io/libpod/busybox:latest \
               docker://localhost:5000/busybox:unsigned

    # ...and pull it back out
    run_skopeo copy --src-cert-dir=$TESTDIR/client-auth \
               docker://localhost:5000/busybox:unsigned \
               dir:$TESTDIR/extracted

    # inspect with cert
    run_skopeo inspect --cert-dir=$TESTDIR/client-auth \
               docker://localhost:5000/busybox:unsigned
    expect_output --substring "localhost:5000/busybox"

    # delete with cert
    run_skopeo delete --cert-dir=$TESTDIR/client-auth \
               docker://localhost:5000/busybox:unsigned
}

teardown() {
    podman rm -f reg

    standard_teardown
}

# vim: filetype=sh
