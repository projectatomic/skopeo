# Signature access protocols

The `github.com/containers/image` library supports signatures implemented as blobs “attached to” an image.
Some image transports (local storage formats and remote procotocols) implement these signatures natively
or trivially; for others, the protocol extensions described below are necessary.

## docker/distribution registries—separate storage

### Usage

Any existing docker/distribution registry, whether or not it natively supports signatures,
can be augmented with separate signature storage by configuring a signature storage URL in [`registries.d`](registries.d.md).
`registries.d` can be configured to use one storage URL for a whole docker/distribution server,
or also separate URLs for smaller namespaces or individual repositories within the server
(which e.g. allows image authors to manage their own signature storage while publishing
the images on the public `docker.io` server).

The signature storage URL defines a root of a path hierarchy.
It can be either a `file:///…` URL, pointing to a local directory structure,
or a `http`/`https` URL, pointing to a remote server.
`file:///` signature storage can be both read and written, `http`/`https` only supports reading.

The same path hierarchy is used in both cases, so the HTTP/HTTPS server can be
a simple static web server serving a directory structure created by writing to a `file:///` signature storage.
(This of course does not prevent other server implementations,
e.g. a HTTP server reading signatures from a database.)

The usual workflow for producing and distributing images using the separate storage mechanism
is to configure the repository in `registries.d` with `sigstore-staging` URL pointing to a private
`file:///` staging area, and a `sigstore` URL pointing to a public web server.
To publish an image, the image author would sign the image as necessary (e.g. using `skopeo copy`),
and then copy the created directory structure from the `file:///` staging area
to a subdirectory of a webroot of the public web server so that they are accessible using the public `sigstore` URL.
The author would also instruct consumers of the image to, or provide a `registries.d` configuration file to,
set up a `sigstore` URL pointing to the public web server.

### Path structure

Given a _base_ signature storage URL configured in `registries.d` as mentioned above,
and a container image stored in a docker/distribution registry using the _fully-expanded_ name
_hostname_`/`_namespaces_`/`_name_{`@`_digest_,`:`_tag_} (e.g. for `docker.io/library/busybox:latest`,
_namespaces_ is `library`, even if the user refers to the image using the shorter syntax as `busybox:latest`),
signatures are accessed using URLs of the form
> _base_`/`_namespaces_`/`_name_`@`_digest-algo_`=`_digest-value_`/signature-`_index_

where _digest-algo_`:`_digest-value_ is a manifest digest usable for referencing the relevant image manifest
(i.e. even if the user referenced the image using a tag,
the signature storage is always disambiguated using digest references).
Note that in the URLs used for signatures,
_digest-algo_ and _digest-value_ are separated using the `=` character,
not `:` like when acessing the manifest using the docker/distribution API.

Within the URL, _index_ is a decimal integer (in the canonical form), starting with 1.
Signatures are stored at URLs with successive _index_ values; to read all of them, start with _index_=1,
and continue reading signatures and increasing _index_ as long as signatures with these _index_ values exist.
Similarly, to add one more signature to an image, find the first _index_ which does not exist, and
then store the new signature using that _index_ value.

There is no way to list existing signatures other than iterating through the successive _index_ values,
and no way to download all of the signatures at once.

### Examples

For a docker/distribution image available as `busybox@sha256:817a12c32a39bbe394944ba49de563e085f1d3c5266eb8e9723256bc4448680e`
(or as `busybox:latest` if the `latest` tag points to to a manifest with the same digest),
and with a `registries.d` configuration specifying a `sigstore` URL `https://example.com/sigstore` for the same image,
the following URLs would be accessed to download all signatures:
> - `https://example.com/sigstore/library/busybox@sha256=817a12c32a39bbe394944ba49de563e085f1d3c5266eb8e9723256bc4448680e/signature-1`
> - `https://example.com/sigstore/library/busybox@sha256=817a12c32a39bbe394944ba49de563e085f1d3c5266eb8e9723256bc4448680e/signature-2`
> - …

For a docker/distribution image available as `example.com/ns1/ns2/ns3/repo@somedigest:digestvalue` and the same
`sigstore` URL, the signatures would be available at
> `https://example.com/sigstore/ns1/ns2/ns3/repo@somedigest=digestvalue/signature-1`

and so on.

## (OpenShift) docker/distribution API extension

As of https://github.com/openshift/origin/pull/12504/ , the OpenShift-embedded registry also provides
an extension of the docker/distribution API which allows simpler access to the signatures,
using only the docker/distribution API endpoint.

This API is not inherently OpenShift-specific (e.g. the client does not need to know the OpenShift API endpoint,
and credentials sufficient to access the docker/distribution API server are sufficient to access signatures as well),
and it is the preferred way implement signature storage in registries.

See https://github.com/openshift/openshift-docs/pull/3556 for the upstream documentation of the API.

To read the signature, any user with access to an image can use the `/extensions/v2/…/signatures/…`
path to read an array of signatures.  Use only the signature objects
which have `version` equal to `2`, `type` equal to `atomic`, and read the signature from `content`;
ignore the other fields of the signature object.

To add a single signature, `PUT` a new object with `version` set to `2`, `type` set to `atomic`,
and `content` set to the signature.  Also set `name` to an unique name with the form
_digest_`@`_per-image-name_, where _digest_ is an image manifest digest (also used in the URL),
and _per-image-name_ is any unique identifier.

To add more than one signature, add them one at a time.  This API does not allow deleting signatures.

Note that because signatures are stored within the cluster-wide image objects,
i.e. different namespaces can not associate different sets of signatures to the same image,
updating signatures requires a cluster-wide access to the `imagesignatures` resource
(by default available to the `system:image-signer` role),

## OpenShift-embedded registries

The OpenShift-embedded registry implements the ordinary docker/distribution API,
and it also exposes images through the OpenShift REST API (available through the “API master” servers).

Note: OpenShift versions 1.5 and later support the above-described [docker/distribution API extension](#openshift-dockerdistribution-api-extension),
which is easier to set up and should usually be preferred.
Continue reading for details on using older versions of OpenShift.

As of https://github.com/openshift/origin/pull/9181,
signatures are exposed through the OpenShift API
(i.e. to access the complete image, it is necessary to use both APIs,
in particular to know the URLs for both the docker/distribution and the OpenShift API master endpoints).

To read the signature, any user with access to an image can use the `imagestreamimages` namespaced
resource to read an `Image` object and its `Signatures` array.  Use only the `ImageSignature` objects
which have `Type` equal to `atomic`, and read the signature from `Content`; ignore the other fields of
the `ImageSignature` object.

To add or remove signatures, use the cluster-wide (non-namespaced) `imagesignatures` resource,
with `Type` set to `atomic` and `Content` set to the signature.  Signature names must have the form
_digest_`@`_per-image-name_, where _digest_ is an image manifest digest (OpenShift “image name”),
and _per-image-name_ is any unique identifier.

Note that because signatures are stored within the cluster-wide image objects,
i.e. different namespaces can not associate different sets of signatures to the same image,
updating signatures requires a cluster-wide access to the `imagesignatures` resource
(by default available to the `system:image-signer` role),
and deleting signatures is strongly discouraged
(it deletes the signature from all namespaces which contain the same image).
