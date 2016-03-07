package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/registry/client/transport"
	dockerdistribution "github.com/docker/docker/distribution"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/image/v1"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	engineTypes "github.com/docker/engine-api/types"
	"github.com/projectatomic/skopeo/types"
	"golang.org/x/net/context"
)

type v1ManifestFetcher struct {
	endpoint    registry.APIEndpoint
	repoInfo    *registry.RepositoryInfo
	repo        distribution.Repository
	confirmedV2 bool
	// wrap in a config?
	authConfig engineTypes.AuthConfig
	service    *registry.Service
	session    *registry.Session
}

func (mf *v1ManifestFetcher) Fetch(ctx context.Context, ref reference.Named) (*types.ImageInspect, error) {
	var (
		imgInspect *types.ImageInspect
	)
	if _, isCanonical := ref.(reference.Canonical); isCanonical {
		// Allowing fallback, because HTTPS v1 is before HTTP v2
		return nil, fallbackError{err: dockerdistribution.ErrNoSupport{errors.New("Cannot pull by digest with v1 registry")}}
	}
	tlsConfig, err := mf.service.TLSConfig(mf.repoInfo.Index.Name)
	if err != nil {
		return nil, err
	}
	// Adds Docker-specific headers as well as user-specified headers (metaHeaders)
	tr := transport.NewTransport(
		registry.NewTransport(tlsConfig),
		//registry.DockerHeaders(mf.config.MetaHeaders)...,
		registry.DockerHeaders(dockerversion.DockerUserAgent(), nil)...,
	)
	client := registry.HTTPClient(tr)
	//v1Endpoint, err := mf.endpoint.ToV1Endpoint(mf.config.MetaHeaders)
	v1Endpoint, err := mf.endpoint.ToV1Endpoint(dockerversion.DockerUserAgent(), nil)
	if err != nil {
		logrus.Debugf("Could not get v1 endpoint: %v", err)
		return nil, fallbackError{err: err}
	}
	mf.session, err = registry.NewSession(client, &mf.authConfig, v1Endpoint)
	if err != nil {
		logrus.Debugf("Fallback from error: %s", err)
		return nil, fallbackError{err: err}
	}
	imgInspect, err = mf.fetchWithSession(ctx, ref)
	if err != nil {
		return nil, err
	}
	return imgInspect, nil
}

func (mf *v1ManifestFetcher) fetchWithSession(ctx context.Context, ref reference.Named) (*types.ImageInspect, error) {
	repoData, err := mf.session.GetRepositoryData(mf.repoInfo)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP code: 404") {
			return nil, fmt.Errorf("Error: image %s not found", mf.repoInfo.RemoteName())
		}
		// Unexpected HTTP error
		return nil, err
	}

	var tagsList map[string]string
	tagsList, err = mf.session.GetRemoteTags(repoData.Endpoints, mf.repoInfo)
	if err != nil {
		logrus.Errorf("unable to get remote tags: %s", err)
		return nil, err
	}

	logrus.Debugf("Retrieving the tag list")
	tagged, isTagged := ref.(reference.NamedTagged)
	var tagID, tag string
	if isTagged {
		tag = tagged.Tag()
		tagsList[tagged.Tag()] = tagID
	} else {
		ref, err = reference.WithTag(ref, reference.DefaultTag)
		if err != nil {
			return nil, err
		}
		tagged, _ := ref.(reference.NamedTagged)
		tag = tagged.Tag()
		tagsList[tagged.Tag()] = tagID
	}
	tagID, err = mf.session.GetRemoteTag(repoData.Endpoints, mf.repoInfo, tag)
	if err == registry.ErrRepoNotFound {
		return nil, fmt.Errorf("Tag %s not found in repository %s", tag, mf.repoInfo.FullName())
	}
	if err != nil {
		logrus.Errorf("unable to get remote tags: %s", err)
		return nil, err
	}

	tagList := []string{}
	for tag := range tagsList {
		tagList = append(tagList, tag)
	}

	img := repoData.ImgList[tagID]

	var pulledImg *image.Image
	for _, ep := range mf.repoInfo.Index.Mirrors {
		if pulledImg, err = mf.pullImageJSON(img.ID, ep, repoData.Tokens); err != nil {
			// Don't report errors when pulling from mirrors.
			logrus.Debugf("Error pulling image json of %s:%s, mirror: %s, %s", mf.repoInfo.FullName(), img.Tag, ep, err)
			continue
		}
		break
	}
	if pulledImg == nil {
		for _, ep := range repoData.Endpoints {
			if pulledImg, err = mf.pullImageJSON(img.ID, ep, repoData.Tokens); err != nil {
				// It's not ideal that only the last error is returned, it would be better to concatenate the errors.
				logrus.Infof("Error pulling image json of %s:%s, endpoint: %s, %v", mf.repoInfo.FullName(), img.Tag, ep, err)
				continue
			}
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("Error pulling image (%s) from %s, %v", img.Tag, mf.repoInfo.FullName(), err)
	}
	if pulledImg == nil {
		return nil, fmt.Errorf("No such image %s:%s", mf.repoInfo.FullName(), tag)
	}

	return makeImageInspect(pulledImg, []byte{}, tag, "", tagList), nil
}

func (mf *v1ManifestFetcher) pullImageJSON(imgID, endpoint string, token []string) (*image.Image, error) {
	imgJSON, _, err := mf.session.GetRemoteImageJSON(imgID, endpoint)
	if err != nil {
		return nil, err
	}
	h, err := v1.HistoryFromConfig(imgJSON, false)
	if err != nil {
		return nil, err
	}
	configRaw, err := makeRawConfigFromV1Config(imgJSON, image.NewRootFS(), []image.History{h})
	if err != nil {
		return nil, err
	}
	config, err := json.Marshal(configRaw)
	if err != nil {
		return nil, err
	}
	img, err := image.NewFromJSON(config)
	if err != nil {
		return nil, err
	}
	return img, nil
}
