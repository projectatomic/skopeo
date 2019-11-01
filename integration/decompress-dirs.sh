#!/bin/bash -e
# Account for differences between dir: images that are solely due to one being
# compressed (fresh from a registry) and the other not being compressed (read
# from storage, which decompressed it and had to reassemble the layer blobs).
for dir in "$@" ; do
    # Updating the manifest's blob digests may change the formatting, so
    # use jq to get them into similar shape.
    jq -M . "${dir}"/manifest.json > "${dir}"/manifest.json.tmp && mv "${dir}"/manifest.json.tmp "${dir}"/manifest.json
    for candidate in "${dir}"/???????????????????????????????????????????????????????????????? ; do
        # If a digest-identified file looks like it was compressed,
        # decompress it, and replace its hash and size in the manifest
        # with the values for their decompressed versions.
        uncompressed=`zcat "${candidate}" 2> /dev/null | sha256sum | cut -c1-64`
        if test $? -eq 0 ; then
            if test "$uncompressed" != e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 ; then
                zcat "${candidate}" > "${dir}"/${uncompressed}
                sed -r -i -e "s#sha256:$(basename ${candidate})#sha256:${uncompressed}#g" "${dir}"/manifest.json
                sed -r -i -e "s#\"size\": $(wc -c < ${candidate}),#\"size\": $(wc -c < ${dir}/${uncompressed}),#g" "${dir}"/manifest.json
                rm -f "${candidate}"
            fi
        fi
    done
done
