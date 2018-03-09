#
# ----- Go Builder Alpine Image ------
#
FROM golang:1.9-alpine AS builder

RUN apk add --no-cache git bash || apk update && apk upgrade
RUN apk add --no-cache make gcc python2 perl || apk update && apk upgrade

RUN apk add --no-cache \
    musl-dev \
    btrfs-progs-dev \
    lvm2-dev \
    gpgme-dev \
    glib-dev || apk update && apk upgrade

RUN apk add --no-cache -X http://nl.alpinelinux.org/alpine/edge/testing ostree-dev

# set working directory
RUN mkdir -p /go/src/github.com/projectatomic/skopeo
WORKDIR /go/src/github.com/projectatomic/skopeo

# copy sources (including .git repo)
COPY . .

# run test and calculate coverage: skip for RELEASE
RUN make binary-local

#
# ------ Pumba runtime image ------
#
FROM alpine:3.6

RUN apk add --no-cache \
    device-mapper-libs \
    gpgme \
    glib \
    expat \
    libacl \
    libarchive \
    libassuan \
    libattr \
    libblkid \
    libbz2 \
    libffi \
    libgpg-error \
    libintl \
    libmount \
    libressl2.5-libcrypto \
    libsoup \
    libuuid \
    libxml2 \
    lz4-libs \
    pcre \
    sqlite-libs \
    xz-libs \
    zlib || apk update && apk upgrade

RUN apk add --no-cache -X http://nl.alpinelinux.org/alpine/edge/testing ostree

COPY --from=builder /go/src/github.com/projectatomic/skopeo/skopeo /usr/local/bin/skopeo

CMD ["/usr/local/bin/skopeo", "--help"]
