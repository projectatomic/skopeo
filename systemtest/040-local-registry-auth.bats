#!/usr/bin/env bats
#
# Tests with a local registry with auth
#

load helpers

function setup() {
    standard_setup

    # Remove old/stale cred file
    _cred_dir=$TESTDIR/credentials
    export XDG_RUNTIME_DIR=$_cred_dir
    mkdir -p $_cred_dir/containers
    rm -f $_cred_dir/containers/auth.json

    # Start authenticated registry with random password
    testuser=testuser
    testpassword=$(random_string 15)

    start_registry --testuser=$testuser --testpassword=$testpassword --enable-delete=true reg
}

@test "auth: credentials on command line" {
    # No creds
    run_skopeo 1 inspect --tls-verify=false docker://localhost:5000/nonesuch
    expect_output --substring "unauthorized: authentication required"

    # Wrong user
    run_skopeo 1 inspect --tls-verify=false --creds=baduser:badpassword \
               docker://localhost:5000/nonesuch
    expect_output --substring "unauthorized: authentication required"

    # Wrong password
    run_skopeo 1 inspect --tls-verify=false --creds=$testuser:badpassword \
               docker://localhost:5000/nonesuch
    expect_output --substring "unauthorized: authentication required"

    # Correct creds, but no such image
    run_skopeo 1 inspect --tls-verify=false --creds=$testuser:$testpassword \
               docker://localhost:5000/nonesuch
    expect_output --substring "manifest unknown: manifest unknown"

    # These should pass
    run_skopeo copy --dest-tls-verify=false --dcreds=$testuser:$testpassword \
               docker://quay.io/libpod/busybox:latest \
               docker://localhost:5000/busybox:mine
    run_skopeo inspect --tls-verify=false --creds=$testuser:$testpassword \
               docker://localhost:5000/busybox:mine
    expect_output --substring "localhost:5000/busybox"
}

@test "auth: credentials via podman login" {
    # Logged in: skopeo should work
    podman login --tls-verify=false -u $testuser -p $testpassword localhost:5000

    run_skopeo copy --dest-tls-verify=false \
               docker://quay.io/libpod/busybox:latest \
               docker://localhost:5000/busybox:mine
    run_skopeo inspect --tls-verify=false docker://localhost:5000/busybox:mine
    expect_output --substring "localhost:5000/busybox"

    # Logged out: should fail
    podman logout localhost:5000

    run_skopeo 1 inspect --tls-verify=false docker://localhost:5000/busybox:mine
    expect_output --substring "unauthorized: authentication required"
}

@test "auth: copy with --src-creds and --dest-creds" {
    run_skopeo copy --dest-tls-verify=false --dest-creds=$testuser:$testpassword \
               docker://quay.io/libpod/busybox:latest \
               docker://localhost:5000/busybox:mine
    run_skopeo copy --src-tls-verify=false --src-creds=$testuser:$testpassword \
               docker://localhost:5000/busybox:mine \
               dir:$TESTDIR/dir1
    run ls $TESTDIR/dir1
    expect_output --substring "manifest.json"
}

@test "auth: credentials via authfile" {
    podman login --tls-verify=false --authfile $TESTDIR/test.auth -u $testuser -p $testpassword localhost:5000

    # copy without authfile: should fail
    run_skopeo 1 copy --dest-tls-verify=false \
               docker://quay.io/libpod/busybox:latest \
               docker://localhost:5000/busybox:mine

    # copy with authfile: should work
    run_skopeo copy --dest-tls-verify=false \
               --authfile $TESTDIR/test.auth \
               docker://quay.io/libpod/busybox:latest \
               docker://localhost:5000/busybox:mine

    # inspect without authfile: should fail
    run_skopeo 1 inspect --tls-verify=false docker://localhost:5000/busybox:mine
    expect_output --substring "unauthorized: authentication required"

    # inspect with authfile: should work
    run_skopeo inspect --tls-verify=false --authfile $TESTDIR/test.auth docker://localhost:5000/busybox:mine
    expect_output --substring "localhost:5000/busybox"

    # delete without authfile: should fail
    run_skopeo 1 delete --tls-verify=false docker://localhost:5000/busybox:mine
    expect_output --substring "authentication required"

    # delete with authfile: should work
    run_skopeo delete --tls-verify=false --authfile $TESTDIR/test.auth docker://localhost:5000/busybox:mine
}

teardown() {
    podman rm -f reg

    if [[ -n $_cred_dir ]]; then
        rm -rf $_cred_dir
    fi

    standard_teardown
}

# vim: filetype=sh
