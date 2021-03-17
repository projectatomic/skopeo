# skopeo container image build with Travis

This document describes the details and requirements to build and publish skopeo container images. The images are published as several architecture specific images and multiarch images on top for upstream and stable versions.

The Travis configuration is available at `.travis.yml`.

The code to build and publish images is available at `release/Makefile` and should be used only via Travis.

Travis workflow has 3 major pieces:
- `local-build` - build and test source code locally on osx and linux/amd64 environments, 2 jobs are running in parallel
- `image-build-push` - build and push container images with several Travis jobs running in parallel to build images for several architectures (linux/amd64, linux/s390x, linux/ppc64le). Build part is done for each PR, push part is executed only in case of cron job or master branch update.
- `manifest-multiarch-push` - create and push image manifests, which consists of architecture specific images from previous step. Executed only in case of cron job or master branch update.

## Ways to have full workflow run
- [cron job](https://docs.travis-ci.com/user/cron-jobs/#adding-cron-jobs)
- Trigger build from Travis CI
- Update code in master branch

## Environment variables

Several environment variables are used to customize image names and keep private credentials to push to quay.io repositories.

**Image tags** are specified in environment variable and should be manually updated in case of new release.

- `SKOPEO_QUAY_USERNAME` and `SKOPEO_QUAY_PASSWORD` are credentials to push images to `quay.io/skopeo/stable` and `quay.io/skopeo/upstream` repos, and require the credentials to have write permissions. These variables should be specified in [Travis](https://docs.travis-ci.com/user/environment-variables/#defining-variables-in-repository-settings).
- `CONTAINERS_QUAY_USERNAME` and `CONTAINERS_QUAY_PASSWORD` are credentials to push images to `quay.io/containers/skopeo` repos, and require the credentials to have write permissions. These variables should be specified in [Travis](https://docs.travis-ci.com/user/environment-variables/#defining-variables-in-repository-settings).

Variables in .travis.yml
- `MULTIARCH_MANIFEST_ARCHITECTURES` is a list with architecture shortnames, to appear in final multiarch manifest. The values should fit to architectures used in the `image-build-push` Travis step.
- `STABLE_IMAGE`, `EXTRA_STABLE_IMAGE` are image names to publish stable Skopeo.
- `UPSTREAM_IMAGE` is an image name to publish upstream Skopeo.

### Values for environment variables

| Env variable                     | Value                            |
| -------------------------------- |----------------------------------|
| MULTIARCH_MANIFEST_ARCHITECTURES | "amd64 s390x ppc64le"            |
| STABLE_IMAGE                     | quay.io/skopeo/stable:v1.2.0     |
| EXTRA_STABLE_IMAGE               | quay.io/containers/skopeo:v1.2.0 |
| UPSTREAM_IMAGE                   | quay.io/skopeo/upstream:master   |
