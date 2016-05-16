package docker

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/projectatomic/skopeo/dockerutils"
	"github.com/projectatomic/skopeo/reference"
	"github.com/projectatomic/skopeo/types"
)

type dockerImageDestination struct {
	ref reference.Named
	tag string
	c   *dockerClient
}

// NewDockerImageDestination creates a new ImageDestination for the specified image and connection specification.
func NewDockerImageDestination(img, certPath string, tlsVerify bool) (types.ImageDestination, error) {
	ref, tag, err := parseDockerImageName(img)
	if err != nil {
		return nil, err
	}
	c, err := newDockerClient(ref.Hostname(), certPath, tlsVerify)
	if err != nil {
		return nil, err
	}
	return &dockerImageDestination{
		ref: ref,
		tag: tag,
		c:   c,
	}, nil
}

func (d *dockerImageDestination) CanonicalDockerReference() (string, error) {
	return fmt.Sprintf("%s:%s", d.ref.Name(), d.tag), nil
}

func (d *dockerImageDestination) PutManifest(manifest []byte) error {
	// FIXME: This only allows upload by digest, not creating a tag.  See the
	// corresponding comment in NewOpenshiftImageDestination.
	digest, err := dockerutils.ManifestDigest(manifest)
	if err != nil {
		return err
	}
	url := fmt.Sprintf(manifestURL, d.ref.RemoteName(), digest)

	headers := map[string]string{}
	mimeType := dockerutils.GuessManifestMIMEType(manifest)
	if mimeType != "" {
		headers["Content-Type"] = mimeType
	}
	res, err := d.c.makeRequest("PUT", url, headers, bytes.NewReader(manifest))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		body, err := ioutil.ReadAll(res.Body)
		if err == nil {
			logrus.Debugf("Error body %s", string(body))
		}
		logrus.Debugf("Error uploading manifest, status %d, %#v", res.StatusCode, res)
		return fmt.Errorf("Error uploading manifest to %s, status %d", url, res.StatusCode)
	}
	return nil
}

func (d *dockerImageDestination) PutLayer(digest string, stream io.Reader) error {
	checkURL := fmt.Sprintf(blobsURL, d.ref.RemoteName(), digest)

	logrus.Debugf("Checking %s", checkURL)
	res, err := d.c.makeRequest("HEAD", checkURL, nil, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK && res.Header.Get("Docker-Content-Digest") == digest {
		logrus.Debugf("... already exists, not uploading")
		return nil
	}
	logrus.Debugf("... failed, status %d", res.StatusCode)

	// FIXME? Chunked upload, progress reporting, etc.
	uploadURL := fmt.Sprintf(blobUploadURL, d.ref.RemoteName(), digest)
	logrus.Debugf("Uploading %s", uploadURL)
	// FIXME: Set Content-Length?
	res, err = d.c.makeRequest("POST", uploadURL, map[string]string{"Content-Type": "application/octet-stream"}, stream)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		logrus.Debugf("Error uploading, status %d", res.StatusCode)
		return fmt.Errorf("Error uploading to %s, status %d", uploadURL, res.StatusCode)
	}

	return nil
}

func (d *dockerImageDestination) PutSignatures(signatures [][]byte) error {
	if len(signatures) != 0 {
		return fmt.Errorf("Pushing signatures to a Docker Registry is not supported")
	}
	return nil
}
