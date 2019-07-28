#!/usr/bin/env bats
#
# Copy tests
#

load helpers

function setup() {
    standard_setup

    start_registry reg
}

# From remote, to dir1, to local, to dir2;
# compare dir1 and dir2, expect no changes
@test "copy: dir, round trip" {
    local remote_image=docker://busybox:latest
    local localimg=docker://localhost:5000/busybox:unsigned

    local dir1=$TESTDIR/dir1
    local dir2=$TESTDIR/dir2

    run_skopeo copy          $remote_image  dir:$dir1
    run_skopeo copy --dest-tls-verify=false dir:$dir1  $localimg
    run_skopeo copy  --src-tls-verify=false            $localimg  dir:$dir2

    # Both extracted copies must be identical
    diff -urN $dir1 $dir2
}

# Same as above, but using 'oci:' instead of 'dir:' and with a :latest tag
@test "copy: oci, round trip" {
    local remote_image=docker://busybox:latest
    local localimg=docker://localhost:5000/busybox:unsigned

    local dir1=$TESTDIR/oci1
    local dir2=$TESTDIR/oci2

    run_skopeo copy          $remote_image  oci:$dir1:latest
    run_skopeo copy --dest-tls-verify=false oci:$dir1:latest  $localimg
    run_skopeo copy  --src-tls-verify=false                   $localimg  oci:$dir2:latest

    # Both extracted copies must be identical
    diff -urN $dir1 $dir2
}

# Same image, extracted once with :tag and once without
@test "copy: oci w/ and w/o tags" {
    local remote_image=docker://busybox:latest

    local dir1=$TESTDIR/dir1
    local dir2=$TESTDIR/dir2

    run_skopeo copy $remote_image oci:$dir1
    run_skopeo copy $remote_image oci:$dir2:withtag

    # Both extracted copies must be identical, except for index.json
    diff -urN --exclude=index.json $dir1 $dir2

    # ...which should differ only in the tag. (But that's too hard to check)
    grep '"org.opencontainers.image.ref.name":"withtag"' $dir2/index.json
}

# This one seems unlikely to get fixed
@test "copy: bug 651" {
    skip "Enable this once skopeo issue #651 has been fixed"

    run_skopeo copy --dest-tls-verify=false \
               docker://quay.io/libpod/alpine_labels:latest \
               docker://localhost:5000/foo
}

# Don't copy the image again if the image already exists in the destination registry
@test "copy: don't copy the same image twice" {
    local output
    local remote_image=docker://busybox:latest
    local localimg=docker://localhost:5000/busybox:unsigned

    run_skopeo copy --dest-tls-verify=false $remote_image $localimg
    output=$(run_skopeo --debug copy --dest-tls-verify=false $remote_image $localimg)
    echo $output | grep "Using blob info cache"
}

teardown() {
    podman rm -f reg

    standard_teardown
}

# vim: filetype=sh
