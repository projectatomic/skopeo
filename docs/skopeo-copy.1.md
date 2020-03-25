% skopeo-copy(1)

## NAME
skopeo\-copy - Copy an image (manifest, filesystem layers, signatures) from one location to another.

## SYNOPSIS
**skopeo copy** [**--sign-by=**_key-ID_] _source-image destination-image_

## DESCRIPTION
Copy an image (manifest, filesystem layers, signatures) from one location to another.

Uses the system's trust policy to validate images, rejects images not trusted by the policy.

  _source-image_ use the "image name" format described above

  _destination-image_ use the "image name" format described above

## OPTIONS

**--all**

If _source-image_ refers to a list of images, instead of copying just the image which matches the current OS and
architecture (subject to the use of the global --override-os, --override-arch and --override-variant options), attempt to copy all of
the images in the list, and the list itself.

**--authfile** _path_

Path of the authentication file. Default is ${XDG_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

**--src-authfile** _path_

Path of the authentication file for the source registry. Uses path given by `--authfile`, if not provided.

**--dest-authfile** _path_

Path of the authentication file for the destination registry. Uses path given by `--authfile`, if not provided.

**--format, -f** _manifest-type_ Manifest type (oci, v2s1, or v2s2) to use when saving image to directory using the 'dir:' transport (default is manifest type of source)

**--quiet, -q** suppress output information when copying images

**--remove-signatures** do not copy signatures, if any, from _source-image_. Necessary when copying a signed image to a destination which does not support signatures.

**--sign-by=**_key-id_ add a signature using that key ID for an image name corresponding to _destination-image_

**--encryption-key** _Key_ a reference prefixed with the encryption protocol to use. The supported protocols are JWE, PGP and PKCS7. For instance, jwe:/path/to/key.pem or pgp:admin@example.com or pkcs7:/path/to/x509-file. This feature is still *experimental*.

**--decryption-key** _Key_ a reference required to perform decryption of container images. This should point to files which represent keys and/or certificates that can be used for decryption. Decryption will be tried with all keys. This feature is still *experimental*.

**--src-creds** _username[:password]_ for accessing the source registry

**--dest-compress** _bool-value_ Compress tarball image layers when saving to directory using the 'dir' transport. (default is same compression type as source)

**--dest-oci-accept-uncompressed-layers** _bool-value_ Allow uncompressed image layers when saving to an OCI image using the 'oci' transport. (default is to compress things that aren't compressed)

**--dest-creds** _username[:password]_ for accessing the destination registry

**--src-cert-dir** _path_ Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the source registry or daemon

**--src-no-creds** _bool-value_ Access the registry anonymously.

**--src-tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to container source registry or daemon (defaults to true)

**--dest-cert-dir** _path_ Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the destination registry or daemon

**--dest-no-creds** _bool-value_  Access the registry anonymously.

**--dest-tls-verify** _bool-value_ Require HTTPS and verify certificates when talking to container destination registry or daemon (defaults to true)

**--src-daemon-host** _host_ Copy from docker daemon at _host_. If _host_ starts with `tcp://`, HTTPS is enabled by default. To use plain HTTP, use the form `http://` (default is `unix:///var/run/docker.sock`).

**--dest-daemon-host** _host_ Copy to docker daemon at _host_. If _host_ starts with `tcp://`, HTTPS is enabled by default. To use plain HTTP, use the form `http://` (default is `unix:///var/run/docker.sock`).

Existing signatures, if any, are preserved as well.

**--dest-compress-format** _format_ Specifies the compression format to use.  Supported values are: `gzip` and `zstd`.

**--dest-compress-level** _format_ Specifies the compression level to use.  The value is specific to the compression algorithm used, e.g. for zstd the accepted values are in the range 1-20 (inclusive), while for gzip it is 1-9 (inclusive).

## EXAMPLES

To copy the layers of the docker.io busybox image to a local directory:
```sh
$ mkdir -p /var/lib/images/busybox
$ skopeo copy docker://busybox:latest dir:/var/lib/images/busybox
$ ls /var/lib/images/busybox/*
  /tmp/busybox/2b8fd9751c4c0f5dd266fcae00707e67a2545ef34f9a29354585f93dac906749.tar
  /tmp/busybox/manifest.json
  /tmp/busybox/8ddc19f16526912237dd8af81971d5e4dd0587907234be2b83e249518d5b673f.tar
```

To copy and sign an image:

```sh
# skopeo copy --sign-by dev@example.com container-storage:example/busybox:streaming docker://example/busybox:gold
```

To encrypt an image:
```sh
skopeo copy docker://docker.io/library/nginx:1.17.8 oci:local_nginx:1.17.8

openssl genrsa -out private.key 1024
openssl rsa -in private.key -pubout > public.key

skopeo  copy --encryption-key jwe:./public.key oci:local_nginx:1.17.8 oci:try-encrypt:encrypted
```

To decrypt an image:
```sh
skopeo copy --decryption-key ./private.key oci:try-encrypt:encrypted oci:try-decrypt:decrypted
```

To copy encrypted image without decryption:
```sh
skopeo copy oci:try-encrypt:encrypted oci:try-encrypt-copy:encrypted
```

To decrypt an image that requires more than one key:
```sh
skopeo copy --decryption-key ./private1.key --decryption-key ./private2.key --decryption-key ./private3.key oci:try-encrypt:encrypted oci:try-decrypt:decrypted
```

Container images can also be partially encrypted by specifying the index of the layer. Layers are 0-indexed indices, with support for negative indexing. i.e. 0 is the first layer, -1 is the last layer.

Let's say out of 3 layers that the image `docker.io/library/nginx:1.17.8` is made up of, we only want to encrypt the 2nd layer,
```sh
skopeo  copy --encryption-key jwe:./public.key --encrypt-layer 1 oci:local_nginx:1.17.8 oci:try-encrypt:encrypted
```

## SEE ALSO
skopeo(1), podman-login(1), docker-login(1)

## AUTHORS

Antonio Murdaca <runcom@redhat.com>, Miloslav Trmac <mitr@redhat.com>, Jhon Honce <jhonce@redhat.com>

