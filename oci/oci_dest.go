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
	"github.com/projectatomic/skopeo/types"
)

type ociManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType"`
	Config        descriptor        `json:"config"`
	Layers        []descriptor      `json:"layers"`
	Annotations   map[string]string `json:"annotations"`
}

type descriptor struct {
	Digest    string `json:"digest"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
}

type ociImageDestination struct {
	dir string
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

func createManifest(m []byte) ([]byte, string, error) {
	om := ociManifest{}
	mt := manifest.GuessMIMEType(m)
	switch mt {
	case manifest.DockerV2Schema1MIMEType:
		// There a simple reason about not yet implementing this.
		// OCI image-spec assure about backward compatibility with docker v2s2 but not v2s1
		// generating a v2s2 is a migration docker does when upgrading to 1.10.3
		// and I don't think we should bother about this now (I don't want to have migration code here in skopeo)
		return nil, "", fmt.Errorf("not yet implemented")
	case manifest.DockerV2Schema2MIMEType:
		if err := json.Unmarshal(m, &om); err != nil {
			return nil, "", err
		}
		om.MediaType = manifest.OCIV1ImageManifestMIMEType
		b, err := json.Marshal(om)
		if err != nil {
			return nil, "", err
		}
		return b, om.MediaType, nil
	case manifest.DockerV2ListMIMEType:
		return nil, "", fmt.Errorf("not yet implemented")
	case manifest.OCIV1ImageManifestListMIMEType:
		return nil, "", fmt.Errorf("not yet implemented")
	case manifest.OCIV1ImageManifestMIMEType:
		return m, om.MediaType, nil
	}
	return nil, "", fmt.Errorf("Unrecognized manifest media type")
}

func (d *ociImageDestination) PutManifest(m []byte) error {
	if err := d.ensureParentDirectoryExists("refs"); err != nil {
		return err
	}
	var err error
	defer func() {
		if err != nil {
			d.cleanup()
		}
	}()
	// TODO(mitr, runcom): this breaks signatures entirely since at this point we're creating a new manifest
	// and signatures don't apply anymore. Will fix.
	ociMan, mt, err := createManifest(m)
	if err != nil {
		return err
	}
	digest, err := manifest.Digest(ociMan)
	if err != nil {
		return err
	}
	desc := descriptor{}
	desc.Digest = digest
	// TODO(runcom): beaware and add support for OCI manifest list
	desc.MediaType = mt
	desc.Size = int64(len(ociMan))
	data, err := json.Marshal(desc)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(blobPath(d.dir, digest), ociMan, 0644); err != nil {
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
	var err error
	defer func() {
		if err != nil {
			d.cleanup()
		}
	}()
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

func (d *ociImageDestination) cleanup() {
	os.RemoveAll(d.dir)
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
