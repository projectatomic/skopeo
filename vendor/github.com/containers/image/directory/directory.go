package directory

import (
	"fmt"
	"path/filepath"
	"strings"
)

// manifestPath returns a path for the manifest within a directory using our conventions.
func manifestPath(dir string) string {
	return filepath.Join(dir, "manifest.json")
}

// layerPath returns a path for a layer tarball within a directory using our conventions.
func layerPath(dir string, digest string) string {
	// FIXME: Should we keep the digest identification?
	return filepath.Join(dir, strings.TrimPrefix(digest, "sha256:")+".tar")
}

// signaturePath returns a path for a signature within a directory using our conventions.
func signaturePath(dir string, index int) string {
	return filepath.Join(dir, fmt.Sprintf("signature-%d", index+1))
}
