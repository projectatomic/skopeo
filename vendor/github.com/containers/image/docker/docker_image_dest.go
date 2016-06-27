package docker

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/containers/image/manifest"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
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

func (d *dockerImageDestination) PutManifest(m []byte) error {
	// FIXME: This only allows upload by digest, not creating a tag.  See the
	// corresponding comment in NewOpenshiftImageDestination.
	digest, err := manifest.Digest(m)
	if err != nil {
		return err
	}
	url := fmt.Sprintf(manifestURL, d.ref.RemoteName(), digest)

	headers := map[string][]string{}
	mimeType := manifest.GuessMIMEType(m)
	if mimeType != "" {
		headers["Content-Type"] = []string{mimeType}
	}
	res, err := d.c.makeRequest("PUT", url, headers, bytes.NewReader(m))
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

func (d *dockerImageDestination) PutBlob(digest string, stream io.Reader) error {
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
	uploadURL := fmt.Sprintf(blobUploadURL, d.ref.RemoteName())
	logrus.Debugf("Uploading %s", uploadURL)
	res, err = d.c.makeRequest("POST", uploadURL, nil, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		logrus.Debugf("Error initiating layer upload, response %#v", *res)
		return fmt.Errorf("Error initiating layer upload to %s, status %d", uploadURL, res.StatusCode)
	}
	uploadLocation, err := res.Location()
	if err != nil {
		return fmt.Errorf("Error determining upload URL: %s", err.Error())
	}

	// FIXME: DELETE uploadLocation on failure

	locationQuery := uploadLocation.Query()
	locationQuery.Set("digest", digest)
	uploadLocation.RawQuery = locationQuery.Encode()
	res, err = d.c.makeRequestToResolvedURL("PUT", uploadLocation.String(), map[string][]string{"Content-Type": {"application/octet-stream"}}, stream)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		logrus.Debugf("Error uploading layer, response %#v", *res)
		return fmt.Errorf("Error uploading layer to %s, status %d", uploadLocation, res.StatusCode)
	}

	logrus.Debugf("Upload of layer %s complete", digest)
	return nil
}

func (d *dockerImageDestination) PutSignatures(signatures [][]byte) error {
	if len(signatures) != 0 {
		return fmt.Errorf("Pushing signatures to a Docker Registry is not supported")
	}
	return nil
}
