package oci

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/projectatomic/skopeo/types"
)

type descriptor struct {
	Digest    string
	MediaType string
	Size      int64
}

type ociImageDestination struct {
	dir string
}

// NewOCIImageDestination returns an ImageDestination for writing to an existing directory.
func NewOCIImageDestination(dir string) types.ImageDestination {
	return &ociImageDestination{dir: dir}
}

func (d *ociImageDestination) CanonicalDockerReference() (string, error) {
	return "", fmt.Errorf("Can not determine canonical Docker reference for an OCI directory")
}

// TODO(runcom): manifest here it's just a image-spec/image.Descriptor and not bytes!!!
func (d *ociImageDestination) PutManifest(manifest []byte) error {
	// TODO(runcom) use manifests.Digest
	digest := sha256.New()
	_, err := digest.Write(manifest)
	if err != nil {
		return err
	}
	desc := descriptor{}
	desc.Digest = hex.EncodeToString(digest.Sum(nil))
	desc.MediaType = "application/vnd.oci.image.manifest.v1+json"
	desc.Size = int64(len(manifest))
	data, err := json.Marshal(desc)
	if err != nil {
		return err
	}
	//return ioutil.WriteFile(manifestPath(d.dir), []byte(`{"imageLayoutVersion": "1.0.0"}`), 0644)

	// TODO(we should name the manifest into OCI - under $DEST/refs/)
	return ioutil.WriteFile(manifestPath(d.dir, "todo"), data, 0644)
}

func (d *ociImageDestination) PutBlob(digest string, stream io.Reader) error {
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

func (d *ociImageDestination) PutSignatures(signatures [][]byte) error {
	// TODO
	return fmt.Errorf("Not implemented")
}

// manifestPath returns a path for the manifest within a directory using our conventions.
func ociLayout(dir string) string {
	return filepath.Join(dir, "oci-layout")
}

// blobPath returns a path for a blob within a directory using OCI image-layout convention.
func blobPath(dir string, digest string) string {
	return filepath.Join(dir, "blobs", strings.Replace(digest, ":", "-", -1))
}

func manifestPath(dir string, digest string) string {
	return filepath.Join(dir, "refs", digest)
}
