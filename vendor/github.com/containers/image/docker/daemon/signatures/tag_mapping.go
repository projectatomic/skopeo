package signatures

// tagMappingBucket records the manifest digest for the latest version of each pulled tag.

// tagMappingBucket keys are (config digest, canonical format of reference.NamedTagged),
// values are manifest digests.

// Safety WRT concurrent access to data:
// We only read/write a single key.  The transaction encompassing the whole write protects us
// against making dangling references visible.

import (
	"bytes"

	"github.com/containers/image/docker/reference"
	bolt "github.com/etcd-io/bbolt"
	digest "github.com/opencontainers/go-digest"
)

var tagMappingBucket = []byte("config+name+tag->manifest")

// tagMappingBucketKey returns a key for use in tagMappingBucket
func tagMappingBucketKey(configDigest digest.Digest, namedTagged reference.NamedTagged) ([]byte, error) {
	configBytes, err := stringToNonNULBytes(configDigest.String())
	if err != nil {
		return nil, err
	}
	ntBytes, err := stringToNonNULBytes(namedTagged.String())
	if err != nil {
		return nil, err
	}
	return bytes.Join([][]byte{configBytes, ntBytes}, []byte{0}), nil
}

// resolveManifestDigest resolves (configDigest, ref) into a manifest digest, potentially reading from tx.
// ref may be nil, a reference.Digested, or a reference.NamedTagged. Other kinds of references (e.g. an IsNameOnly() reference) are silently ignored.
// (If the caller uses a default tag, it is the caller’s responsibility to supply the tagged variant of the reference.)
// Returns ("", nil) if nothing was found; error is reported only on failures reading or invalid inpupt.
func resolveManifestDigest(tx *bolt.Tx, configDigest digest.Digest, ref reference.Reference) (digest.Digest, error) {
	if ref == nil {
		return "", nil
	}
	if digested, ok := ref.(reference.Digested); ok { // reference.Digested does not even require a Name(); we need only the digest.
		return digested.Digest(), nil
	}
	if nt, ok := ref.(reference.NamedTagged); ok {
		key, err := tagMappingBucketKey(configDigest, nt)
		if err != nil {
			return "", err
		}
		b := tx.Bucket(tagMappingBucket)
		if b != nil {
			mdBytes := b.Get(key)
			if mdBytes != nil {
				md, err := digest.Parse(string(mdBytes))
				if err != nil {
					return "", err
				}
				return md, nil
			}
		}
	}
	return "", nil
}

// updateTagMapping updates tagMappingBucket, if necessary, to point (configDigest, ref) at manifestDigest
func updateTagMapping(tx *bolt.Tx, configDigest digest.Digest, ref reference.NamedTagged, manifestDigest digest.Digest) error {
	if ref != nil {
		key, err := tagMappingBucketKey(configDigest, ref)
		if err != nil {
			return err
		}
		b, err := tx.CreateBucketIfNotExists(tagMappingBucket)
		if err != nil {
			return err
		}
		// This may overwrite a previous value at the same location; that’s fine, we have fetched an updated image from the same tag.
		return b.Put(key, []byte(manifestDigest.String()))
	}
	return nil
}
