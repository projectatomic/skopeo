#!/usr/bin/env bash
set -e

cd "$(dirname "$BASH_SOURCE")/.."
rm -rf vendor/
source 'hack/.vendor-helpers.sh'

clone git github.com/urfave/cli v1.17.0
clone git github.com/containers/image master
clone git gopkg.in/cheggaaa/pb.v1 ad4efe000aa550bb54918c06ebbadc0ff17687b9 https://github.com/cheggaaa/pb
clone git github.com/Sirupsen/logrus v0.10.0
clone git github.com/go-check/check v1
clone git github.com/stretchr/testify v1.1.3
clone git github.com/davecgh/go-spew master
clone git github.com/pmezard/go-difflib master
# docker deps from https://github.com/docker/docker/blob/v1.11.2/hack/vendor.sh
clone git github.com/docker/docker v1.12.1
clone git github.com/docker/engine-api 4eca04ae18f4f93f40196a17b9aa6e11262a7269
clone git github.com/docker/go-connections 4ccf312bf1d35e5dbda654e57a9be4c3f3cd0366
clone git github.com/vbatts/tar-split v0.9.11
clone git github.com/gorilla/context 14f550f51a
clone git github.com/gorilla/mux e444e69cbd
clone git github.com/docker/go-units 651fc226e7441360384da338d0fd37f2440ffbe3
clone git golang.org/x/net master https://github.com/golang/net.git
# end docker deps
clone git github.com/docker/distribution 07f32ac1831ed0fc71960b7da5d6bb83cb6881b5
clone git github.com/docker/libtrust master
clone git github.com/opencontainers/runc master
clone git github.com/opencontainers/image-spec 7dc1ee39c59c6949612c6fdf502a4727750cb063
clone git github.com/mtrmac/gpgme master
# openshift/origin' k8s dependencies as of OpenShift v1.1.5
clone git github.com/golang/glog 44145f04b68cf362d9c4df2182967c2275eaefed
clone git k8s.io/kubernetes 4a3f9c5b19c7ff804cbc1bf37a15c044ca5d2353 https://github.com/openshift/kubernetes
clone git github.com/ghodss/yaml 73d445a93680fa1a78ae23a5839bad48f32ba1ee
clone git gopkg.in/yaml.v2 d466437aa4adc35830964cffc5b5f262c63ddcb4
clone git github.com/imdario/mergo 6633656539c1639d9d78127b7d47c622b5d7b6dc

clean

mv vendor/src/* vendor/
