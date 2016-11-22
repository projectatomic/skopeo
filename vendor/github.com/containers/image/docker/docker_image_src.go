package docker

import (
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/docker/distribution/registry/client"
)

type dockerImageSource struct {
	ref                        dockerReference
	requestedManifestMIMETypes []string
	c                          *dockerClient
	// State
	cachedManifest         []byte // nil if not loaded yet
	cachedManifestMIMEType string // Only valid if cachedManifest != nil
}

// newImageSource creates a new ImageSource for the specified image reference,
// asking the backend to use a manifest from requestedManifestMIMETypes if possible.
// nil requestedManifestMIMETypes means manifest.DefaultRequestedManifestMIMETypes.
// The caller must call .Close() on the returned ImageSource.
func newImageSource(ctx *types.SystemContext, ref dockerReference, requestedManifestMIMETypes []string) (*dockerImageSource, error) {
	c, err := newDockerClient(ctx, ref, false)
	if err != nil {
		return nil, err
	}
	if requestedManifestMIMETypes == nil {
		requestedManifestMIMETypes = manifest.DefaultRequestedManifestMIMETypes
	}
	return &dockerImageSource{
		ref: ref,
		requestedManifestMIMETypes: requestedManifestMIMETypes,
		c: c,
	}, nil
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *dockerImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *dockerImageSource) Close() {
}

// simplifyContentType drops parameters from a HTTP media type (see https://tools.ietf.org/html/rfc7231#section-3.1.1.1)
// Alternatively, an empty string is returned unchanged, and invalid values are "simplified" to an empty string.
func simplifyContentType(contentType string) string {
	if contentType == "" {
		return contentType
	}
	mimeType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return mimeType
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
func (s *dockerImageSource) GetManifest() ([]byte, string, error) {
	err := s.ensureManifestIsLoaded()
	if err != nil {
		return nil, "", err
	}
	return s.cachedManifest, s.cachedManifestMIMEType, nil
}

func (s *dockerImageSource) fetchManifest(tagOrDigest string) ([]byte, string, error) {
	url := fmt.Sprintf(manifestURL, s.ref.ref.RemoteName(), tagOrDigest)
	headers := make(map[string][]string)
	headers["Accept"] = s.requestedManifestMIMETypes
	res, err := s.c.makeRequest("GET", url, headers, nil)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", client.HandleErrorResponse(res)
	}
	manblob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	return manblob, simplifyContentType(res.Header.Get("Content-Type")), nil
}

// GetTargetManifest returns an image's manifest given a digest.
// This is mainly used to retrieve a single image's manifest out of a manifest list.
func (s *dockerImageSource) GetTargetManifest(digest string) ([]byte, string, error) {
	return s.fetchManifest(digest)
}

// ensureManifestIsLoaded sets s.cachedManifest and s.cachedManifestMIMEType
//
// ImageSource implementations are not required or expected to do any caching,
// but because our signatures are “attached” to the manifest digest,
// we need to ensure that the digest of the manifest returned by GetManifest
// and used by GetSignatures are consistent, otherwise we would get spurious
// signature verification failures when pulling while a tag is being updated.
func (s *dockerImageSource) ensureManifestIsLoaded() error {
	if s.cachedManifest != nil {
		return nil
	}

	reference, err := s.ref.tagOrDigest()
	if err != nil {
		return err
	}

	manblob, mt, err := s.fetchManifest(reference)
	if err != nil {
		return err
	}
	// We might validate manblob against the Docker-Content-Digest header here to protect against transport errors.
	s.cachedManifest = manblob
	s.cachedManifestMIMEType = mt
	return nil
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
func (s *dockerImageSource) GetBlob(digest string) (io.ReadCloser, int64, error) {
	url := fmt.Sprintf(blobsURL, s.ref.ref.RemoteName(), digest)
	logrus.Debugf("Downloading %s", url)
	res, err := s.c.makeRequest("GET", url, nil, nil)
	if err != nil {
		return nil, 0, err
	}
	if res.StatusCode != http.StatusOK {
		// print url also
		return nil, 0, fmt.Errorf("Invalid status code returned when fetching blob %d", res.StatusCode)
	}
	size, err := strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		size = -1
	}
	return res.Body, size, nil
}

func (s *dockerImageSource) GetSignatures() ([][]byte, error) {
	if s.c.signatureBase == nil { // Skip dealing with the manifest digest if not necessary.
		return [][]byte{}, nil
	}

	if err := s.ensureManifestIsLoaded(); err != nil {
		return nil, err
	}
	manifestDigest, err := manifest.Digest(s.cachedManifest)
	if err != nil {
		return nil, err
	}

	signatures := [][]byte{}
	for i := 0; ; i++ {
		url := signatureStorageURL(s.c.signatureBase, manifestDigest, i)
		if url == nil {
			return nil, fmt.Errorf("Internal error: signatureStorageURL with non-nil base returned nil")
		}
		signature, missing, err := s.getOneSignature(url)
		if err != nil {
			return nil, err
		}
		if missing {
			break
		}
		signatures = append(signatures, signature)
	}
	return signatures, nil
}

// getOneSignature downloads one signature from url.
// If it successfully determines that the signature does not exist, returns with missing set to true and error set to nil.
func (s *dockerImageSource) getOneSignature(url *url.URL) (signature []byte, missing bool, err error) {
	switch url.Scheme {
	case "file":
		logrus.Debugf("Reading %s", url.Path)
		sig, err := ioutil.ReadFile(url.Path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, true, nil
			}
			return nil, false, err
		}
		return sig, false, nil

	case "http", "https":
		logrus.Debugf("GET %s", url)
		res, err := s.c.client.Get(url.String())
		if err != nil {
			return nil, false, err
		}
		defer res.Body.Close()
		if res.StatusCode == http.StatusNotFound {
			return nil, true, nil
		} else if res.StatusCode != http.StatusOK {
			return nil, false, fmt.Errorf("Error reading signature from %s: status %d", url.String(), res.StatusCode)
		}
		sig, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, false, err
		}
		return sig, false, nil

	default:
		return nil, false, fmt.Errorf("Unsupported scheme when reading signature from %s", url.String())
	}
}

// deleteImage deletes the named image from the registry, if supported.
func deleteImage(ctx *types.SystemContext, ref dockerReference) error {
	c, err := newDockerClient(ctx, ref, true)
	if err != nil {
		return err
	}

	// When retrieving the digest from a registry >= 2.3 use the following header:
	//   "Accept": "application/vnd.docker.distribution.manifest.v2+json"
	headers := make(map[string][]string)
	headers["Accept"] = []string{manifest.DockerV2Schema2MediaType}

	reference, err := ref.tagOrDigest()
	if err != nil {
		return err
	}
	getURL := fmt.Sprintf(manifestURL, ref.ref.RemoteName(), reference)
	get, err := c.makeRequest("GET", getURL, headers, nil)
	if err != nil {
		return err
	}
	defer get.Body.Close()
	manifestBody, err := ioutil.ReadAll(get.Body)
	if err != nil {
		return err
	}
	switch get.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return fmt.Errorf("Unable to delete %v. Image may not exist or is not stored with a v2 Schema in a v2 registry", ref.ref)
	default:
		return fmt.Errorf("Failed to delete %v: %s (%v)", ref.ref, manifestBody, get.Status)
	}

	digest := get.Header.Get("Docker-Content-Digest")
	deleteURL := fmt.Sprintf(manifestURL, ref.ref.RemoteName(), digest)

	// When retrieving the digest from a registry >= 2.3 use the following header:
	//   "Accept": "application/vnd.docker.distribution.manifest.v2+json"
	delete, err := c.makeRequest("DELETE", deleteURL, headers, nil)
	if err != nil {
		return err
	}
	defer delete.Body.Close()

	body, err := ioutil.ReadAll(delete.Body)
	if err != nil {
		return err
	}
	if delete.StatusCode != http.StatusAccepted {
		return fmt.Errorf("Failed to delete %v: %s (%v)", deleteURL, string(body), delete.Status)
	}

	if c.signatureBase != nil {
		manifestDigest, err := manifest.Digest(manifestBody)
		if err != nil {
			return err
		}

		for i := 0; ; i++ {
			url := signatureStorageURL(c.signatureBase, manifestDigest, i)
			if url == nil {
				return fmt.Errorf("Internal error: signatureStorageURL with non-nil base returned nil")
			}
			missing, err := c.deleteOneSignature(url)
			if err != nil {
				return err
			}
			if missing {
				break
			}
		}
	}

	return nil
}
