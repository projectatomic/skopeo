package signatures

// dataBucket stores the original manifests and signatures for an image.

// dataBucket keys are (config digest, manifest digest), and contains a sub-bucket for each key.
// This sub-bucket stores a manifest in manifestKey, and individual signatures in
// (signatureKeyPrefix + zero-based index.)

// Safety WRT concurrent access to data in a dataBucket sub-bucket:
// The bucket is identified by the manifest digest, so no substantial updates to the
// manifest are ever expected.
// Readers expect signatures stored within a sub-bucket to be replaced/updated atomically
// to ensure the bucket contents are consistent.

import (
	"bytes"
	"fmt"
	"strconv"

	bolt "github.com/etcd-io/bbolt"
	digest "github.com/opencontainers/go-digest"
)

var (
	dataBucket         = []byte("config+manifest->bucket")
	manifestKey        = []byte("manifest")
	signatureKeyPrefix = []byte("sig")
)

// dataBucketKey returns a key for use in dataBucket.
func dataBucketKey(configDigest, manifestDigest digest.Digest) ([]byte, error) {
	configBytes, err := stringToNonNULBytes(configDigest.String())
	if err != nil {
		return nil, err
	}
	manifestBytes, err := stringToNonNULBytes(manifestDigest.String())
	if err != nil {
		return nil, err
	}
	return bytes.Join([][]byte{configBytes, manifestBytes}, []byte{0}), nil
}

// copyBytes returns a freshly allocated clone of input.
// This is needed because the data pointers returned by boltdb are invalid after the end of the transaction.
func copyBytes(input []byte) []byte {
	res := make([]byte, len(input))
	copy(res, input)
	return res
}

// readManifest returns the original manifest stored in b, or nil if not available.
func readManifest(b *bolt.Bucket) []byte {
	m := b.Get(manifestKey)
	if m == nil {
		return nil
	}
	return copyBytes(m)
}

// writeManifest stores the original manifest to b.
func writeManifest(b *bolt.Bucket, manifest []byte) error {
	return b.Put(manifestKey, manifest)
}

// readSignatures returns the original signatures stored in bucket, which is dataKey.
func readSignatures(bucket *bolt.Bucket, dataKey []byte) ([][]byte, error) {
	// Iterate through all keys in the sub-bucket; we need all of the except for manifestKey, so it seems fastest to read them in the database order and then reorder in memory.
	signatureMap := map[int][]byte{}
	if err := bucket.ForEach(func(k, v []byte) error {
		if !bytes.HasPrefix(k, signatureKeyPrefix) {
			return nil
		}
		i, err := strconv.Atoi(string(bytes.TrimPrefix(k, signatureKeyPrefix)))
		if err != nil {
			return err
		}
		if _, ok := signatureMap[i]; ok {
			return fmt.Errorf("Internal error: Duplicate key %q in dataBucket key %q", k, dataKey)
		}
		signatureMap[i] = copyBytes(v)
		return nil
	}); err != nil {
		return nil, err
	}

	signatures := [][]byte{}
	for i := 0; ; i++ {
		signature, ok := signatureMap[i]
		if !ok {
			break
		}
		signatures = append(signatures, signature)
	}
	if len(signatures) != len(signatureMap) {
		// The use of transactions to update signatures should prevent this from happening
		return nil, fmt.Errorf("Internal error: Non-consecutive signatures in dataBucket key %q", dataKey)
	}
	return signatures, nil
}

// writeSignatures stores the original signatures to b, which is dataKey.
func writeSignatures(b *bolt.Bucket, dataKey []byte, signatures [][]byte) error {
	if len(signatures) == 0 {
		return nil // Don't bother reading the old ones.
	}

	existingSigs, err := readSignatures(b, dataKey)
	if err != nil {
		return err
	}

	nextIndex := len(existingSigs)
sigExists:
	for _, sig := range signatures {
		for _, existingSig := range existingSigs {
			if bytes.Equal(sig, existingSig) {
				continue sigExists
			}
		}

		key := bytes.Join([][]byte{signatureKeyPrefix, []byte(strconv.Itoa(nextIndex))}, []byte{})
		if err := b.Put(key, sig); err != nil {
			return err
		}
		nextIndex++
	}
	return nil
}
