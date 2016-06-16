#!/usr/bin/env bash
set -e

cd "$(dirname "$BASH_SOURCE")/.."
rm -rf vendor/
source 'hack/.vendor-helpers.sh'

clone git github.com/urfave/cli v1.17.0
clone git github.com/Sirupsen/logrus v0.10.0
clone git github.com/go-check/check v1
clone git github.com/stretchr/testify v1.1.3
clone git github.com/davecgh/go-spew master
clone git github.com/pmezard/go-difflib master
clone git github.com/docker/docker 0f5c9d301b9b1cca66b3ea0f9dec3b5317d3686d
clone git github.com/docker/distribution master
clone git github.com/docker/libtrust master
clone git github.com/opencontainers/runc master
clone git github.com/mtrmac/gpgme master
# openshift/origin' k8s dependencies as of OpenShift v1.1.5
clone git github.com/golang/glog 44145f04b68cf362d9c4df2182967c2275eaefed
clone git k8s.io/kubernetes 4a3f9c5b19c7ff804cbc1bf37a15c044ca5d2353 https://github.com/openshift/kubernetes
clone git github.com/ghodss/yaml 73d445a93680fa1a78ae23a5839bad48f32ba1ee
clone git gopkg.in/yaml.v2 d466437aa4adc35830964cffc5b5f262c63ddcb4
clone git github.com/imdario/mergo 6633656539c1639d9d78127b7d47c622b5d7b6dc

clean

mv vendor/src/* vendor/
