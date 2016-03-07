
package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/client"
	dockerdistribution "github.com/docker/docker/distribution"
	"github.com/docker/docker/image"
	"github.com/docker/docker/image/v1"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	engineTypes "github.com/docker/engine-api/types"
	"github.com/runcom/skopeo/types"
	"golang.org/x/net/context"
)

type v2ManifestFetcher struct {
	endpoint    registry.APIEndpoint
	repoInfo    *registry.RepositoryInfo
	repo        distribution.Repository
	confirmedV2 bool
	// wrap in a config?
	authConfig engineTypes.AuthConfig
	service    *registry.Service
}

func (mf *v2ManifestFetcher) Fetch(ctx context.Context, ref reference.Named) (*types.ImageInspect, error) {
	var (
		imgInspect *types.ImageInspect
		err        error
	)

	//mf.repo, mf.confirmedV2, err = distribution.NewV2Repository(ctx, mf.repoInfo, mf.endpoint, mf.config.MetaHeaders, mf.config.AuthConfig, "pull")
	mf.repo, mf.confirmedV2, err = dockerdistribution.NewV2Repository(ctx, mf.repoInfo, mf.endpoint, nil, &mf.authConfig, "pull")
	if err != nil {
		logrus.Debugf("Error getting v2 registry: %v", err)
		return nil, err
	}

	imgInspect, err = mf.fetchWithRepository(ctx, ref)
	if err != nil {
		if _, ok := err.(fallbackError); ok {
			return nil, err
		}
		if continueOnError(err) {
			logrus.Errorf("Error trying v2 registry: %v", err)
			return nil, fallbackError{err: err, confirmedV2: mf.confirmedV2, transportOK: true}
		}
	}
	return imgInspect, err
}

func (mf *v2ManifestFetcher) fetchWithRepository(ctx context.Context, ref reference.Named) (*types.ImageInspect, error) {
	var (
		manifest    distribution.Manifest
		tagOrDigest string // Used for logging/progress only
		tagList     = []string{}
	)

	manSvc, err := mf.repo.Manifests(ctx)
	if err != nil {
		return nil, err
	}

	if _, isTagged := ref.(reference.NamedTagged); !isTagged {
		ref, err = reference.WithTag(ref, reference.DefaultTag)
		if err != nil {
			return nil, err
		}
	}

	if tagged, isTagged := ref.(reference.NamedTagged); isTagged {
		// NOTE: not using TagService.Get, since it uses HEAD requests
		// against the manifests endpoint, which are not supported by
		// all registry versions.
		manifest, err = manSvc.Get(ctx, "", client.WithTag(tagged.Tag()))
		if err != nil {
			return nil, allowV1Fallback(err)
		}
		tagOrDigest = tagged.Tag()
	} else if digested, isDigested := ref.(reference.Canonical); isDigested {
		manifest, err = manSvc.Get(ctx, digested.Digest())
		if err != nil {
			return nil, err
		}
		tagOrDigest = digested.Digest().String()
	} else {
		return nil, fmt.Errorf("internal error: reference has neither a tag nor a digest: %s", ref.String())
	}

	if manifest == nil {
		return nil, fmt.Errorf("image manifest does not exist for tag or digest %q", tagOrDigest)
	}

	// If manSvc.Get succeeded, we can be confident that the registry on
	// the other side speaks the v2 protocol.
	mf.confirmedV2 = true

	_, payload, err := manifest.Payload()
	if err != nil {
		return nil, err
	}

	tagList, err = mf.repo.Tags(ctx).All(ctx)
	if err != nil {
		// If this repository doesn't exist on V2, we should
		// permit a fallback to V1.
		return nil, allowV1Fallback(err)
	}

	var (
		image          *image.Image
		manifestDigest digest.Digest
	)

	switch v := manifest.(type) {
	case *schema1.SignedManifest:
		image, manifestDigest, err = mf.pullSchema1(ctx, ref, v)
		if err != nil {
			return nil, err
		}
	case *schema2.DeserializedManifest:
		image, manifestDigest, err = mf.pullSchema2(ctx, ref, v)
		if err != nil {
			return nil, err
		}
	case *manifestlist.DeserializedManifestList:
		image, manifestDigest, err = mf.pullManifestList(ctx, ref, v)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported manifest format")
	}

	// TODO(runcom)
	//var showTags bool
	//if reference.IsNameOnly(ref) {
	//showTags = true
	//logrus.Debug("Using default tag: latest")
	//ref = reference.WithDefaultTag(ref)
	//}
	//_ = showTags
	return makeImageInspect(image, payload, tagOrDigest, manifestDigest, tagList), nil
}

func (mf *v2ManifestFetcher) pullSchema1(ctx context.Context, ref reference.Named, unverifiedManifest *schema1.SignedManifest) (img *image.Image, manifestDigest digest.Digest, err error) {
	var verifiedManifest *schema1.Manifest
	verifiedManifest, err = verifySchema1Manifest(unverifiedManifest, ref)
	if err != nil {
		return nil, "", err
	}

	// remove duplicate layers and check parent chain validity
	err = fixManifestLayers(verifiedManifest)
	if err != nil {
		return nil, "", err
	}

	// Image history converted to the new format
	var history []image.History

	// Note that the order of this loop is in the direction of bottom-most
	// to top-most, so that the downloads slice gets ordered correctly.
	for i := len(verifiedManifest.FSLayers) - 1; i >= 0; i-- {
		var throwAway struct {
			ThrowAway bool `json:"throwaway,omitempty"`
		}
		if err := json.Unmarshal([]byte(verifiedManifest.History[i].V1Compatibility), &throwAway); err != nil {
			return nil, "", err
		}

		h, err := v1.HistoryFromConfig([]byte(verifiedManifest.History[i].V1Compatibility), throwAway.ThrowAway)
		if err != nil {
			return nil, "", err
		}
		history = append(history, h)
	}

	rootFS := image.NewRootFS()
	configRaw, err := makeRawConfigFromV1Config([]byte(verifiedManifest.History[0].V1Compatibility), rootFS, history)

	config, err := json.Marshal(configRaw)
	if err != nil {
		return nil, "", err
	}

	img, err = image.NewFromJSON(config)
	if err != nil {
		return nil, "", err
	}

	manifestDigest = digest.FromBytes(unverifiedManifest.Canonical)

	return img, manifestDigest, nil
}

func verifySchema1Manifest(signedManifest *schema1.SignedManifest, ref reference.Named) (m *schema1.Manifest, err error) {
	// If pull by digest, then verify the manifest digest. NOTE: It is
	// important to do this first, before any other content validation. If the
	// digest cannot be verified, don't even bother with those other things.
	if digested, isCanonical := ref.(reference.Canonical); isCanonical {
		verifier, err := digest.NewDigestVerifier(digested.Digest())
		if err != nil {
			return nil, err
		}
		if _, err := verifier.Write(signedManifest.Canonical); err != nil {
			return nil, err
		}
		if !verifier.Verified() {
			err := fmt.Errorf("image verification failed for digest %s", digested.Digest())
			logrus.Error(err)
			return nil, err
		}
	}
	m = &signedManifest.Manifest

	if m.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported schema version %d for %q", m.SchemaVersion, ref.String())
	}
	if len(m.FSLayers) != len(m.History) {
		return nil, fmt.Errorf("length of history not equal to number of layers for %q", ref.String())
	}
	if len(m.FSLayers) == 0 {
		return nil, fmt.Errorf("no FSLayers in manifest for %q", ref.String())
	}
	return m, nil
}

func fixManifestLayers(m *schema1.Manifest) error {
	imgs := make([]*image.V1Image, len(m.FSLayers))
	for i := range m.FSLayers {
		img := &image.V1Image{}

		if err := json.Unmarshal([]byte(m.History[i].V1Compatibility), img); err != nil {
			return err
		}

		imgs[i] = img
		if err := v1.ValidateID(img.ID); err != nil {
			return err
		}
	}

	if imgs[len(imgs)-1].Parent != "" && runtime.GOOS != "windows" {
		// Windows base layer can point to a base layer parent that is not in manifest.
		return errors.New("Invalid parent ID in the base layer of the image.")
	}

	// check general duplicates to error instead of a deadlock
	idmap := make(map[string]struct{})

	var lastID string
	for _, img := range imgs {
		// skip IDs that appear after each other, we handle those later
		if _, exists := idmap[img.ID]; img.ID != lastID && exists {
			return fmt.Errorf("ID %+v appears multiple times in manifest", img.ID)
		}
		lastID = img.ID
		idmap[lastID] = struct{}{}
	}

	// backwards loop so that we keep the remaining indexes after removing items
	for i := len(imgs) - 2; i >= 0; i-- {
		if imgs[i].ID == imgs[i+1].ID { // repeated ID. remove and continue
			m.FSLayers = append(m.FSLayers[:i], m.FSLayers[i+1:]...)
			m.History = append(m.History[:i], m.History[i+1:]...)
		} else if imgs[i].Parent != imgs[i+1].ID {
			return fmt.Errorf("Invalid parent ID. Expected %v, got %v.", imgs[i+1].ID, imgs[i].Parent)
		}
	}

	return nil
}

func (mf *v2ManifestFetcher) pullSchema2(ctx context.Context, ref reference.Named, mfst *schema2.DeserializedManifest) (img *image.Image, manifestDigest digest.Digest, err error) {
	manifestDigest, err = schema2ManifestDigest(ref, mfst)
	if err != nil {
		return nil, "", err
	}

	target := mfst.Target()

	configChan := make(chan []byte, 1)
	errChan := make(chan error, 1)
	var cancel func()
	ctx, cancel = context.WithCancel(ctx)

	// Pull the image config
	go func() {
		configJSON, err := mf.pullSchema2ImageConfig(ctx, target.Digest)
		if err != nil {
			errChan <- ImageConfigPullError{Err: err}
			cancel()
			return
		}
		configChan <- configJSON
	}()

	var (
		configJSON         []byte      // raw serialized image config
		unmarshalledConfig image.Image // deserialized image config
	)
	if runtime.GOOS == "windows" {
		configJSON, unmarshalledConfig, err = receiveConfig(configChan, errChan)
		if err != nil {
			return nil, "", err
		}
		if unmarshalledConfig.RootFS == nil {
			return nil, "", errors.New("image config has no rootfs section")
		}
	}

	if configJSON == nil {
		configJSON, unmarshalledConfig, err = receiveConfig(configChan, errChan)
		if err != nil {
			return nil, "", err
		}
	}

	img, err = image.NewFromJSON(configJSON)
	if err != nil {
		return nil, "", err
	}

	return img, manifestDigest, nil
}

func (mf *v2ManifestFetcher) pullSchema2ImageConfig(ctx context.Context, dgst digest.Digest) (configJSON []byte, err error) {
	blobs := mf.repo.Blobs(ctx)
	configJSON, err = blobs.Get(ctx, dgst)
	if err != nil {
		return nil, err
	}

	// Verify image config digest
	verifier, err := digest.NewDigestVerifier(dgst)
	if err != nil {
		return nil, err
	}
	if _, err := verifier.Write(configJSON); err != nil {
		return nil, err
	}
	if !verifier.Verified() {
		err := fmt.Errorf("image config verification failed for digest %s", dgst)
		logrus.Error(err)
		return nil, err
	}

	return configJSON, nil
}

func receiveConfig(configChan <-chan []byte, errChan <-chan error) ([]byte, image.Image, error) {
	select {
	case configJSON := <-configChan:
		var unmarshalledConfig image.Image
		if err := json.Unmarshal(configJSON, &unmarshalledConfig); err != nil {
			return nil, image.Image{}, err
		}
		return configJSON, unmarshalledConfig, nil
	case err := <-errChan:
		return nil, image.Image{}, err
		// Don't need a case for ctx.Done in the select because cancellation
		// will trigger an error in p.pullSchema2ImageConfig.
	}
}

// ImageConfigPullError is an error pulling the image config blob
// (only applies to schema2).
type ImageConfigPullError struct {
	Err error
}

// Error returns the error string for ImageConfigPullError.
func (e ImageConfigPullError) Error() string {
	return "error pulling image configuration: " + e.Err.Error()
}

// allowV1Fallback checks if the error is a possible reason to fallback to v1
// (even if confirmedV2 has been set already), and if so, wraps the error in
// a fallbackError with confirmedV2 set to false. Otherwise, it returns the
// error unmodified.
func allowV1Fallback(err error) error {
	switch v := err.(type) {
	case errcode.Errors:
		if len(v) != 0 {
			if v0, ok := v[0].(errcode.Error); ok && shouldV2Fallback(v0) {
				return fallbackError{err: err, confirmedV2: false, transportOK: true}
			}
		}
	case errcode.Error:
		if shouldV2Fallback(v) {
			return fallbackError{err: err, confirmedV2: false, transportOK: true}
		}
	}
	return err
}

// schema2ManifestDigest computes the manifest digest, and, if pulling by
// digest, ensures that it matches the requested digest.
func schema2ManifestDigest(ref reference.Named, mfst distribution.Manifest) (digest.Digest, error) {
	_, canonical, err := mfst.Payload()
	if err != nil {
		return "", err
	}

	// If pull by digest, then verify the manifest digest.
	if digested, isDigested := ref.(reference.Canonical); isDigested {
		verifier, err := digest.NewDigestVerifier(digested.Digest())
		if err != nil {
			return "", err
		}
		if _, err := verifier.Write(canonical); err != nil {
			return "", err
		}
		if !verifier.Verified() {
			err := fmt.Errorf("manifest verification failed for digest %s", digested.Digest())
			logrus.Error(err)
			return "", err
		}
		return digested.Digest(), nil
	}

	return digest.FromBytes(canonical), nil
}

// pullManifestList handles "manifest lists" which point to various
// platform-specifc manifests.
func (mf *v2ManifestFetcher) pullManifestList(ctx context.Context, ref reference.Named, mfstList *manifestlist.DeserializedManifestList) (img *image.Image, manifestListDigest digest.Digest, err error) {
	manifestListDigest, err = schema2ManifestDigest(ref, mfstList)
	if err != nil {
		return nil, "", err
	}

	var manifestDigest digest.Digest
	for _, manifestDescriptor := range mfstList.Manifests {
		// TODO(aaronl): The manifest list spec supports optional
		// "features" and "variant" fields. These are not yet used.
		// Once they are, their values should be interpreted here.
		if manifestDescriptor.Platform.Architecture == runtime.GOARCH && manifestDescriptor.Platform.OS == runtime.GOOS {
			manifestDigest = manifestDescriptor.Digest
			break
		}
	}

	if manifestDigest == "" {
		return nil, "", errors.New("no supported platform found in manifest list")
	}

	manSvc, err := mf.repo.Manifests(ctx)
	if err != nil {
		return nil, "", err
	}

	manifest, err := manSvc.Get(ctx, manifestDigest)
	if err != nil {
		return nil, "", err
	}

	manifestRef, err := reference.WithDigest(ref, manifestDigest)
	if err != nil {
		return nil, "", err
	}

	switch v := manifest.(type) {
	case *schema1.SignedManifest:
		img, _, err = mf.pullSchema1(ctx, manifestRef, v)
		if err != nil {
			return nil, "", err
		}
	case *schema2.DeserializedManifest:
		img, _, err = mf.pullSchema2(ctx, manifestRef, v)
		if err != nil {
			return nil, "", err
		}
	default:
		return nil, "", errors.New("unsupported manifest format")
	}

	return img, manifestListDigest, err
}
