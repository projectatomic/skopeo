package signatures

// signatures.Store allows recording metadata (manifests and signatures)
// which make it possible to authenticate docker images even if the daemon does not store this data itself.

// The store uses github.com/boltdb/bolt, organized into top-level tagMappingBucket and dataBucket.
// See tag_mapping.go and data.go for details about how the two buckets are used.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	bolt "github.com/etcd-io/bbolt"
	digest "github.com/opencontainers/go-digest"
)

// systemDockerSignatureDBPath is the path to the docker signature storage, used for recording metadata allowing to authenticate docker images.
// You can override this at build time with
// -ldflags '-X github.com/containers/image/docker/daemon/signatures.systemDockerSignatureDBPath=$your_path'
var systemDockerSignatureDBPath = builtinDockerSignatureDBPath

// builtinDockerSignatureDBPath is the path to the signature storage.
// DO NOT change this, instead see systemDockerSignatureDBPath above.
const builtinDockerSignatureDBPath = "/var/lib/docker-signatures.boltdb"

// Store allows recording metadata (manifests and signatures) which make it possible to authenticate docker images even if the daemon does not store this data itself.
type Store struct {
	signatureDBPath string
}

// dockerSignatureDBPath returns a path to the signature storage.
func dockerSignatureDBPath(ctx *types.SystemContext) string {
	if ctx != nil {
		if ctx.DockerSignatureDBPath != "" {
			return ctx.DockerSignatureDBPath
		}
		if ctx.RootForImplicitAbsolutePaths != "" {
			return filepath.Join(ctx.RootForImplicitAbsolutePaths, systemDockerSignatureDBPath)
		}
	}
	return systemDockerSignatureDBPath
}

// NewStore returns a Store appropriate for ctx.
func NewStore(ctx *types.SystemContext) *Store {
	return &Store{signatureDBPath: dockerSignatureDBPath(ctx)}
}

// stringToNonNULBytes converts s into a []byte, ensuring it does not contain a byte(0)
func stringToNonNULBytes(s string) ([]byte, error) {
	res := []byte(s)
	if bytes.IndexByte(res, byte(0)) != -1 {
		return nil, fmt.Errorf("Can not use string %q as database key because it contains a NUL byte", s)
	}
	return res, nil
}

// Read reads the manifest and signatures for the specified digest and ref.
// ref may be nil, a reference.Digested, or a reference.NamedTagged. Other kinds of references (e.g. an IsNameOnly() reference) are silently ignored.
// (If the caller uses a default tag, it is the caller’s responsibility to supply the tagged variant of the reference.)
// The returned manifest is nil, and signatures are empty, if not available.
func (s *Store) Read(configDigest digest.Digest, ref reference.Reference) (_ []byte, _ [][]byte, retErr error) {
	db, err := bolt.Open(s.signatureDBPath, 0600, &bolt.Options{ReadOnly: true})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	defer func() {
		err = db.Close()
		if err != nil && retErr == nil {
			retErr = err
		}
	}()

	var manifest []byte
	var sigs [][]byte
	if err := db.View(func(tx *bolt.Tx) error {
		m, s, err := readFromTx(tx, configDigest, ref)
		if err != nil {
			return err
		}
		manifest = m
		sigs = s
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return manifest, sigs, nil
}

// readFromTx reads the manifest and signatures for the specified digest and ref from tx.
// ref may be nil, a reference.Digested, or a reference.NamedTagged. Other kinds of references (e.g. an IsNameOnly() reference) are silently ignored.
// (If the caller uses a default tag, it is the caller’s responsibility to supply the tagged variant of the reference.)
// The returned manifest is nil, and signatures are empty, if not available.
func readFromTx(tx *bolt.Tx, configDigest digest.Digest, ref reference.Reference) ([]byte, [][]byte, error) {
	manifestDigest, err := resolveManifestDigest(tx, configDigest, ref)
	if err != nil {
		return nil, nil, err
	}
	if manifestDigest == "" {
		return nil, nil, nil
	}

	dataKey, err := dataBucketKey(configDigest, manifestDigest)
	if err != nil {
		return nil, nil, err
	}
	b := tx.Bucket(dataBucket)
	if b == nil {
		return nil, nil, nil
	}
	b = b.Bucket(dataKey)
	if b == nil {
		return nil, nil, nil
	}

	manifest := readManifest(b)
	sigs, err := readSignatures(b, dataKey)
	if err != nil {
		return nil, nil, err
	}
	return manifest, sigs, nil
}

// Write stores manifest and signatures for an image with the specified config digest and ref.
// ref may be nil.
func (s *Store) Write(configDigest digest.Digest, ref reference.NamedTagged, manifestBlob []byte, sigs [][]byte) (retErr error) {
	db, err := bolt.Open(s.signatureDBPath, 0600, nil)
	if err != nil {
		return err
	}
	defer func() {
		err = db.Close()
		if err != nil && retErr == nil {
			retErr = err
		}
	}()

	return db.Update(func(tx *bolt.Tx) error {
		return writeToTx(tx, configDigest, ref, manifestBlob, sigs)
	})
}

// Write stores manifest and signatures for an image with the specified config digest and ref to tx.
// ref may be nil.
func writeToTx(tx *bolt.Tx, configDigest digest.Digest, ref reference.NamedTagged, manifestBlob []byte, sigs [][]byte) error {
	// Use the canonical digest algorithm, which is the only one really used.
	// In principle we could use the algorithm from the user-specified ref,
	// but if there are ever multiple algorithms in wide use, we would need to calculate
	// all of the digests from manifestBlob in here anyway, so the user-specified
	// one would not really need to be treated specially.
	manifestDigest, err := manifest.Digest(manifestBlob)
	if err != nil {
		return err
	}

	dataKey, err := dataBucketKey(configDigest, manifestDigest)
	if err != nil {
		return err
	}
	b, err := tx.CreateBucketIfNotExists(dataBucket)
	if err != nil {
		return err
	}
	b, err = b.CreateBucketIfNotExists(dataKey)
	if err != nil {
		return err
	}

	if err := writeManifest(b, manifestBlob); err != nil {
		return err
	}
	if err := writeSignatures(b, dataKey, sigs); err != nil {
		return err
	}
	return updateTagMapping(tx, configDigest, ref, manifestDigest)
}

// RecordImage stores the manifest and signatures of img, which may then be discarded by the caller.
func (s *Store) RecordImage(ctx context.Context, img types.Image) error {
	configDigest := img.ConfigInfo().Digest
	if configDigest == "" {
		return nil // FIXME?! We do not record anything for v2s1?!
	}
	manifest, _, err := img.Manifest(ctx)
	if err != nil {
		return err
	}
	sigs, err := img.Signatures(ctx)
	if err != nil {
		return err
	}

	var namedTaggedRef reference.NamedTagged
	if dockerReference := img.Reference().DockerReference(); dockerReference != nil {
		// dockerReference could in theory also be reference.Canonical (name@digest), which we can ignore:
		// s.Prepare() computes the digest from manifest directly, and the name alone is not useful.
		if nt, ok := dockerReference.(reference.NamedTagged); ok {
			namedTaggedRef = nt
		}
	}
	return s.Write(configDigest, namedTaggedRef, manifest, sigs)
}
