#!/usr/bin/env bats
#
# Simplest test for skopeo inspect
#

load helpers

@test "inspect: basic" {
    workdir=$TESTDIR/inspect

    remote_image=docker://quay.io/libpod/alpine_labels:latest
    # Inspect remote source, then pull it. There's a small race condition
    # in which the remote image can get updated between the inspect and
    # the copy; let's just not worry about it.
    run_skopeo inspect $remote_image
    inspect_remote=$output

    # Now pull it into a directory
    run_skopeo copy $remote_image dir:$workdir
    expect_output --substring "Getting image source signatures"
    expect_output --substring "Writing manifest to image destination"

    # Unpacked contents must include a manifest and version
    [ -e $workdir/manifest.json ]
    [ -e $workdir/version ]

    # Now run inspect locally
    run_skopeo inspect dir:$workdir
    inspect_local=$output

    # Each SHA-named file must be listed in the output of 'inspect'
    for sha in $(find $workdir -type f | xargs -l1 basename | egrep '^[0-9a-f]{64}$'); do
        expect_output --from="$inspect_local" --substring "sha256:$sha" \
                      "Locally-extracted SHA file is present in 'inspect'"
    done

    # Simple sanity check on 'inspect' output.
    # For each of the given keys (LHS of the table below):
    #    1) Get local and remote values
    #    2) Sanity-check local value using simple expression
    #    3) Confirm that local and remote values match.
    #
    # The reason for (2) is to make sure that we don't compare bad results
    #
    # The reason for a hardcoded list, instead of 'jq keys', is that RepoTags
    # is always empty locally, but a list remotely.
    while IFS=$' \t\n' read key expect; do
        local=$(echo "$inspect_local" | jq -r ".$key")
        remote=$(echo "$inspect_remote" | jq -r ".$key")

        expect_output --from="$local" --substring "$expect" \
                  "local $key is sane"

        expect_output --from="$remote" "$local" \
                      "local $key matches remote"
    done <<END_EXPECT
Architecture       amd64
Created            [0-9-]+T[0-9:]+\.[0-9]+Z
Digest             sha256:[0-9a-f]{64}
DockerVersion      [0-9]+\.[0-9][0-9.-]+
Labels             \\\{.*PODMAN.*podman.*\\\}
Layers             \\\[.*sha256:.*\\\]
Os                 linux
END_EXPECT
}

@test "inspect: env" {
    remote_image=docker://quay.io/libpod/fedora:31
    run_skopeo inspect $remote_image
    inspect_remote=$output

    # Simple check on 'inspect' output with environment variables.
    #    1) Get remote image values of environment variables (the value of 'Env')
    #    2) Confirm substring in check_array and the value of 'Env' match.
    check_array=(FGC=f31 DISTTAG=f31container)
    remote=$(jq '.Env[]' <<<"$inspect_remote")
    for substr in ${check_array[@]}; do
        expect_output --from="$remote" --substring "$substr"
    done
}

# Tests https://github.com/containers/skopeo/pull/708
@test "inspect: image manifest list w/ diff platform" {
    # This image's manifest is for an os + arch that is... um, unlikely
    # to support skopeo in the foreseeable future. Or past. The image
    # is created by the make-noarch-manifest script in this directory.
    img=docker://quay.io/libpod/notmyarch:20210121

    # Get our host arch (what we're running on). This assumes that skopeo
    # arch matches podman; it also assumes running podman >= April 2020
    # (prior to that, the format keys were lower-case).
    arch=$(podman info --format '{{.Host.Arch}}')

    # By default, 'inspect' tries to match our host os+arch. This should fail.
    run_skopeo 1 inspect $img
    expect_output --substring "Error parsing manifest for image: Error choosing image instance: no image found in manifest list for architecture $arch, variant " \
                  "skopeo inspect, without --raw, fails"

    # With --raw, we can inspect
    run_skopeo inspect --raw $img
    expect_output --substring "manifests.*platform.*architecture" \
                  "skopeo inspect --raw returns reasonable output"

    # ...and what we get should be consistent with what our script created.
    archinfo=$(jq -r '.manifests[0].platform | {os,variant,architecture} | join("-")' <<<"$output")

    expect_output --from="$archinfo" "amigaos-1000-mc68000" \
                  "os - variant - architecture of $img"
}

# vim: filetype=sh
