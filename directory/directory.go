package directory

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/projectatomic/skopeo/types"
)

// manifestPath returns a path for the manifest within a directory using our conventions.
func manifestPath(dir string) string {
	return filepath.Join(dir, "manifest.json")
}

// manifestPath returns a path for a layer tarball within a directory using our conventions.
func layerPath(dir string, digest string) string {
	// FIXME: Should we keep the digest identification?
	return filepath.Join(dir, strings.TrimPrefix(digest, "sha256:")+".tar")
}

// manifestPath returns a path for a signature within a directory using our conventions.
func signaturePath(dir string, index int) string {
	return filepath.Join(dir, fmt.Sprintf("signature-%d", index+1))
}

type dirImageDestination struct {
	dir string
}

// NewDirImageDestination returns an ImageDestination for writing to an existing directory.
func NewDirImageDestination(dir string) types.ImageDestination {
	return &dirImageDestination{dir}
}

func (d *dirImageDestination) CanonicalDockerReference() (string, error) {
	return "", fmt.Errorf("Can not determine canonical Docker reference for a local directory")
}

func (d *dirImageDestination) PutManifest(manifest []byte) error {
	return ioutil.WriteFile(manifestPath(d.dir), manifest, 0644)
}

func (d *dirImageDestination) PutLayer(digest string, stream io.Reader) error {
	layerFile, err := os.Create(layerPath(d.dir, digest))
	if err != nil {
		return err
	}
	defer layerFile.Close()
	if _, err := io.Copy(layerFile, stream); err != nil {
		return err
	}
	if err := layerFile.Sync(); err != nil {
		return err
	}
	return nil
}

func (d *dirImageDestination) PutSignatures(signatures [][]byte) error {
	for i, sig := range signatures {
		if err := ioutil.WriteFile(signaturePath(d.dir, i), sig, 0644); err != nil {
			return err
		}
	}
	return nil
}

type dirImageSource struct {
	dir string
}

// NewDirImageSource returns an ImageSource reading from an existing directory.
func NewDirImageSource(dir string) types.ImageSource {
	return &dirImageSource{dir}
}

func (s *dirImageSource) GetManifest() (types.ImageManifest, string, error) {
	manifest, err := ioutil.ReadFile(manifestPath(s.dir))
	if err != nil {
		return nil, "", err
	}
	// TODO(runcom): we should hide this type from the pkg API
	m := &types.DockerImageManifest{RawManifest: manifest}

	return m, "", nil // FIXME? unverifiedCanonicalDigest value - really primarily used by dockerImage
}

func (s *dirImageSource) GetLayer(digest string) (io.ReadCloser, error) {
	return os.Open(layerPath(s.dir, digest))
}

func (s *dirImageSource) GetSignatures() ([][]byte, error) {
	signatures := [][]byte{}
	for i := 0; ; i++ {
		signature, err := ioutil.ReadFile(signaturePath(s.dir, i))
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return nil, err
		}
		signatures = append(signatures, signature)
	}
	return signatures, nil
}
