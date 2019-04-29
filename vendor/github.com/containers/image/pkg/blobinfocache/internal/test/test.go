// Package test provides generic BlobInfoCache test helpers.
package test

import (
	"testing"

	"github.com/containers/image/internal/testing/mocks"

	"github.com/containers/image/types"
	digest "github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
)

const (
	digestUnknown             = digest.Digest("sha256:1111111111111111111111111111111111111111111111111111111111111111")
	digestUncompressed        = digest.Digest("sha256:2222222222222222222222222222222222222222222222222222222222222222")
	digestCompressedA         = digest.Digest("sha256:3333333333333333333333333333333333333333333333333333333333333333")
	digestCompressedB         = digest.Digest("sha256:4444444444444444444444444444444444444444444444444444444444444444")
	digestCompressedUnrelated = digest.Digest("sha256:5555555555555555555555555555555555555555555555555555555555555555")
	digestCompressedPrimary   = digest.Digest("sha256:6666666666666666666666666666666666666666666666666666666666666666")
)

// GenericCache runs an implementation-independent set of tests, given a
// newTestCache, which can be called repeatedly and always returns a (cache, cleanup callback) pair
func GenericCache(t *testing.T, newTestCache func(t *testing.T) (types.BlobInfoCache, func(t *testing.T))) {
	for _, s := range []struct {
		name string
		fn   func(t *testing.T, cache types.BlobInfoCache)
	}{
		{"UncompressedDigest", testGenericUncompressedDigest},
		{"RecordDigestUncompressedPair", testGenericRecordDigestUncompressedPair},
		{"RecordKnownLocations", testGenericRecordKnownLocations},
		{"CandidateLocations", testGenericCandidateLocations},
	} {
		t.Run(s.name, func(t *testing.T) {
			cache, cleanup := newTestCache(t)
			defer cleanup(t)
			s.fn(t, cache)
		})
	}
}

func testGenericUncompressedDigest(t *testing.T, cache types.BlobInfoCache) {
	// Nothing is known.
	assert.Equal(t, digest.Digest(""), cache.UncompressedDigest(digestUnknown))

	cache.RecordDigestUncompressedPair(digestCompressedA, digestUncompressed)
	cache.RecordDigestUncompressedPair(digestCompressedB, digestUncompressed)
	// Known compressed→uncompressed mapping
	assert.Equal(t, digestUncompressed, cache.UncompressedDigest(digestCompressedA))
	assert.Equal(t, digestUncompressed, cache.UncompressedDigest(digestCompressedB))
	// This implicitly marks digestUncompressed as uncompressed.
	assert.Equal(t, digestUncompressed, cache.UncompressedDigest(digestUncompressed))

	// Known uncompressed→self mapping
	cache.RecordDigestUncompressedPair(digestCompressedUnrelated, digestCompressedUnrelated)
	assert.Equal(t, digestCompressedUnrelated, cache.UncompressedDigest(digestCompressedUnrelated))
}

func testGenericRecordDigestUncompressedPair(t *testing.T, cache types.BlobInfoCache) {
	for i := 0; i < 2; i++ { // Record the same data twice to ensure redundant writes don’t break things.
		// Known compressed→uncompressed mapping
		cache.RecordDigestUncompressedPair(digestCompressedA, digestUncompressed)
		assert.Equal(t, digestUncompressed, cache.UncompressedDigest(digestCompressedA))
		// Two mappings to the same uncompressed digest
		cache.RecordDigestUncompressedPair(digestCompressedB, digestUncompressed)
		assert.Equal(t, digestUncompressed, cache.UncompressedDigest(digestCompressedB))

		// Mapping an uncompresesd digest to self
		cache.RecordDigestUncompressedPair(digestUncompressed, digestUncompressed)
		assert.Equal(t, digestUncompressed, cache.UncompressedDigest(digestUncompressed))
	}
}

func testGenericRecordKnownLocations(t *testing.T, cache types.BlobInfoCache) {
	transport := mocks.NameImageTransport("==BlobInfocache transport mock")
	for i := 0; i < 2; i++ { // Record the same data twice to ensure redundant writes don’t break things.
		for _, scopeName := range []string{"A", "B"} { // Run the test in two different scopes to verify they don't affect each other.
			scope := types.BICTransportScope{Opaque: scopeName}
			for _, digest := range []digest.Digest{digestCompressedA, digestCompressedB} { // Two different digests should not affect each other either.
				lr1 := types.BICLocationReference{Opaque: scopeName + "1"}
				lr2 := types.BICLocationReference{Opaque: scopeName + "2"}
				cache.RecordKnownLocation(transport, scope, digest, lr2)
				cache.RecordKnownLocation(transport, scope, digest, lr1)
				assert.Equal(t, []types.BICReplacementCandidate{
					{Digest: digest, Location: lr1},
					{Digest: digest, Location: lr2},
				}, cache.CandidateLocations(transport, scope, digest, false))
			}
		}
	}
}

// candidate is a shorthand for types.BICReplacementCandiddate
type candidate struct {
	d  digest.Digest
	lr string
}

func assertCandidatesMatch(t *testing.T, scopeName string, expected []candidate, actual []types.BICReplacementCandidate) {
	e := make([]types.BICReplacementCandidate, len(expected))
	for i, ev := range expected {
		e[i] = types.BICReplacementCandidate{Digest: ev.d, Location: types.BICLocationReference{Opaque: scopeName + ev.lr}}
	}
	assert.Equal(t, e, actual)
}

func testGenericCandidateLocations(t *testing.T, cache types.BlobInfoCache) {
	transport := mocks.NameImageTransport("==BlobInfocache transport mock")
	cache.RecordDigestUncompressedPair(digestCompressedA, digestUncompressed)
	cache.RecordDigestUncompressedPair(digestCompressedB, digestUncompressed)
	cache.RecordDigestUncompressedPair(digestUncompressed, digestUncompressed)
	digestNameSet := []struct {
		n string
		d digest.Digest
	}{
		{"U", digestUncompressed},
		{"A", digestCompressedA},
		{"B", digestCompressedB},
		{"CU", digestCompressedUnrelated},
	}

	for _, scopeName := range []string{"A", "B"} { // Run the test in two different scopes to verify they don't affect each other.
		scope := types.BICTransportScope{Opaque: scopeName}
		// Nothing is known.
		assert.Equal(t, []types.BICReplacementCandidate{}, cache.CandidateLocations(transport, scope, digestUnknown, false))
		assert.Equal(t, []types.BICReplacementCandidate{}, cache.CandidateLocations(transport, scope, digestUnknown, true))

		// Record "2" entries before "1" entries; then results should sort "1" (more recent) before "2" (older)
		for _, suffix := range []string{"2", "1"} {
			for _, e := range digestNameSet {
				cache.RecordKnownLocation(transport, scope, e.d, types.BICLocationReference{Opaque: scopeName + e.n + suffix})
			}
		}

		// No substitutions allowed:
		for _, e := range digestNameSet {
			assertCandidatesMatch(t, scopeName, []candidate{
				{d: e.d, lr: e.n + "1"}, {d: e.d, lr: e.n + "2"},
			}, cache.CandidateLocations(transport, scope, e.d, false))
		}

		// With substitutions: The original digest is always preferred, then other compressed, then the uncompressed one.
		assertCandidatesMatch(t, scopeName, []candidate{
			{d: digestCompressedA, lr: "A1"}, {d: digestCompressedA, lr: "A2"},
			{d: digestCompressedB, lr: "B1"}, {d: digestCompressedB, lr: "B2"},
			{d: digestUncompressed, lr: "U1"}, // Beyond the replacementAttempts limit: {d: digestUncompressed, lr: "U2"},
		}, cache.CandidateLocations(transport, scope, digestCompressedA, true))

		assertCandidatesMatch(t, scopeName, []candidate{
			{d: digestCompressedB, lr: "B1"}, {d: digestCompressedB, lr: "B2"},
			{d: digestCompressedA, lr: "A1"}, {d: digestCompressedA, lr: "A2"},
			{d: digestUncompressed, lr: "U1"}, // Beyond the replacementAttempts limit: {d: digestUncompressed, lr: "U2"},
		}, cache.CandidateLocations(transport, scope, digestCompressedB, true))

		assertCandidatesMatch(t, scopeName, []candidate{
			{d: digestUncompressed, lr: "U1"}, {d: digestUncompressed, lr: "U2"},
			// "1" entries were added after "2", and A/Bs are sorted in the reverse of digestNameSet order
			{d: digestCompressedB, lr: "B1"},
			{d: digestCompressedA, lr: "A1"},
			{d: digestCompressedB, lr: "B2"},
			// Beyond the replacementAttempts limit: {d: digestCompressedA, lr: "A2"},
		}, cache.CandidateLocations(transport, scope, digestUncompressed, true))

		// Locations are known, but no relationships
		assertCandidatesMatch(t, scopeName, []candidate{
			{d: digestCompressedUnrelated, lr: "CU1"}, {d: digestCompressedUnrelated, lr: "CU2"},
		}, cache.CandidateLocations(transport, scope, digestCompressedUnrelated, true))

	}
}
