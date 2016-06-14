package oci

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/projectatomic/skopeo/manifest"
	"github.com/projectatomic/skopeo/reference"
	"github.com/projectatomic/skopeo/types"
)

type descriptor struct {
	Digest    string `json:"digest"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
}

type ociImageDestination struct {
	dir string
	ref reference.Named
	tag string
}

var refRegexp = regexp.MustCompile(`^([A-Za-z0-9._-]+)+$`)

// NewOCIImageDestination returns an ImageDestination for writing to an existing directory.
func NewOCIImageDestination(dir string) (types.ImageDestination, error) {
	dest := dir
	sep := strings.LastIndex(dir, ":")
	tag := "latest"
	if sep != -1 {
		dest = dir[:sep]
		tag = dir[sep+1:]
		if !refRegexp.MatchString(tag) {
			return nil, fmt.Errorf("Invalid reference %s", tag)
		}
	}
	return &ociImageDestination{
		dir: dest,
		tag: tag,
	}, nil
}

func (d *ociImageDestination) CanonicalDockerReference() (string, error) {
	return "", fmt.Errorf("Can not determine canonical Docker reference for an OCI image")
}

func (d *ociImageDestination) PutManifest(m []byte) error {
	if err := d.ensureParentDirectoryExists("refs"); err != nil {
		return err
	}
	// TODO(runcom):
	// make a function to create an OCI manifest from a distribution one which comes here!!!
	// convert the manifest to OCI v1 - beaware we can also receive a docker v2s1 manifest so we need to
	// handle this! docker.io/library/busybox it's still on v2s1 for instance
	digest, err := manifest.Digest(m)
	if err != nil {
		return err
	}
	desc := descriptor{}
	desc.Digest = digest
	mt := manifest.GuessMIMEType(m)
	// TODO(runcom): beaware and add support for OCI manifest list
	desc.MediaType = mt
	desc.Size = int64(len(m))
	data, err := json.Marshal(desc)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(blobPath(d.dir, digest), m, 0644); err != nil {
		return err
	}
	// TODO(runcom): ugly here?
	if err := ioutil.WriteFile(ociLayoutPath(d.dir), []byte(`{"imageLayoutVersion": "1.0.0"}`), 0644); err != nil {
		return err
	}
	return ioutil.WriteFile(manifestPath(d.dir, d.tag), data, 0644)
}

func (d *ociImageDestination) PutBlob(digest string, stream io.Reader) error {
	if err := d.ensureParentDirectoryExists("blobs"); err != nil {
		return err
	}
	blob, err := os.Create(blobPath(d.dir, digest))
	if err != nil {
		return err
	}
	defer blob.Close()
	if _, err := io.Copy(blob, stream); err != nil {
		return err
	}
	if err := blob.Sync(); err != nil {
		return err
	}
	return nil
}

func (d *ociImageDestination) ensureParentDirectoryExists(parent string) error {
	path := filepath.Join(d.dir, parent)
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0700); err != nil {
			return err
		}
	}
	return nil
}

func (d *ociImageDestination) PutSignatures(signatures [][]byte) error {
	if len(signatures) != 0 {
		return fmt.Errorf("Pushing signatures for OCI images is not supported")
	}
	return nil
}

// ociLayoutPathPath returns a path for the oci-layout within a directory using OCI conventions.
func ociLayoutPath(dir string) string {
	return filepath.Join(dir, "oci-layout")
}

// blobPath returns a path for a blob within a directory using OCI image-layout conventions.
func blobPath(dir string, digest string) string {
	return filepath.Join(dir, "blobs", strings.Replace(digest, ":", "-", -1))
}

// manifestPath returns a path for the manifest within a directory using OCI conventions.
func manifestPath(dir string, digest string) string {
	return filepath.Join(dir, "refs", digest)
}
