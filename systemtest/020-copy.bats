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

# Compression zstd
@test "copy: oci, round trip, zstd" {
    local remote_image=docker://busybox:latest

    local dir=$TESTDIR/dir

    run_skopeo copy --dest-compress --dest-compress-format=zstd $remote_image oci:$dir:latest

    # zstd magic number
    local magic=$(printf "\x28\xb5\x2f\xfd")

    # Check there is at least one file that has the zstd magic number as the first 4 bytes
    (for i in $dir/blobs/sha256/*; do test "$(head -c 4 $i)" = $magic && exit 0; done; exit 1)
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

# Registry -> storage -> oci-archive
@test "copy: registry -> storage -> oci-archive" {
    local alpine=docker.io/library/alpine:latest
    local tmp=$TESTDIR/oci

    run_skopeo copy docker://$alpine containers-storage:$alpine
    run_skopeo copy containers-storage:$alpine oci-archive:$tmp
}

# This one seems unlikely to get fixed
@test "copy: bug 651" {
    skip "Enable this once skopeo issue #651 has been fixed"

    run_skopeo copy --dest-tls-verify=false \
               docker://quay.io/libpod/alpine_labels:latest \
               docker://localhost:5000/foo
}

teardown() {
    podman rm -f reg

    standard_teardown
}

# vim: filetype=sh
