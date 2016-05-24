package docker

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/projectatomic/skopeo/docker/utils"
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
	digest, err := utils.ManifestDigest(manifest)
	if err != nil {
		return err
	}
	url := fmt.Sprintf(manifestURL, d.ref.RemoteName(), digest)

	headers := map[string][]string{}
	mimeType := utils.GuessManifestMIMEType(manifest)
	if mimeType != "" {
		headers["Content-Type"] = []string{mimeType}
	}
	res, err := d.c.makeRequest("PUT", url, headers, bytes.NewReader(manifest))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("Error uploading manifest to %s, status %d, %#v, %s", url, res.StatusCode, res, body)
	}
	return nil
}

func (d *dockerImageDestination) PutLayer(digest string, stream io.Reader) error {
	checkURL := fmt.Sprintf(blobsURL, d.ref.RemoteName(), digest)

	res, err := d.c.makeRequest("HEAD", checkURL, nil, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK && res.Header.Get("Docker-Content-Digest") == digest {
		// already exists, not uploading
		return nil
	}

	// FIXME? Chunked upload, progress reporting, etc.
	uploadURL := fmt.Sprintf(blobUploadURL, d.ref.RemoteName(), digest)
	// FIXME: Set Content-Length?
	res, err = d.c.makeRequest("POST", uploadURL, map[string][]string{"Content-Type": {"application/octet-stream"}}, stream)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
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
