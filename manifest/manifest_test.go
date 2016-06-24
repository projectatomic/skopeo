package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuessMIMEType(t *testing.T) {
	cases := []struct {
		path     string
		mimeType string
	}{
		{"ociv1.manifest.json", OCIV1ImageManifestMIMEType},
		{"ociv1list.manifest.json", OCIV1ImageManifestListMIMEType},
		{"v2s2.manifest.json", DockerV2Schema2MIMEType},
		{"v2list.manifest.json", DockerV2ListMIMEType},
		{"v2s1.manifest.json", DockerV2Schema1MIMEType},
		{"v2s1-invalid-signatures.manifest.json", DockerV2Schema1MIMEType},
		{"v2s2nomime.manifest.json", DockerV2Schema2MIMEType}, // It is unclear whether this one is legal, but we should guess v2s2 if anything at all.
		{"unknown-version.manifest.json", ""},
		{"non-json.manifest.json", ""}, // Not a manifest (nor JSON) at all
	}

	for _, c := range cases {
		manifest, err := ioutil.ReadFile(filepath.Join("fixtures", c.path))
		require.NoError(t, err)
		mimeType := GuessMIMEType(manifest)
		assert.Equal(t, c.mimeType, mimeType)
	}
}

func TestDigest(t *testing.T) {
	cases := []struct {
		path   string
		digest string
	}{
		{"v2s2.manifest.json", TestDockerV2S2ManifestDigest},
		{"v2s1.manifest.json", TestDockerV2S1ManifestDigest},
	}
	for _, c := range cases {
		manifest, err := ioutil.ReadFile(filepath.Join("fixtures", c.path))
		require.NoError(t, err)
		digest, err := Digest(manifest)
		require.NoError(t, err)
		assert.Equal(t, c.digest, digest)
	}

	manifest, err := ioutil.ReadFile("fixtures/v2s1-invalid-signatures.manifest.json")
	require.NoError(t, err)
	digest, err := Digest(manifest)
	assert.Error(t, err)

	digest, err = Digest([]byte{})
	require.NoError(t, err)
	assert.Equal(t, "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", digest)
}

func TestMatchesDigest(t *testing.T) {
	cases := []struct {
		path   string
		digest string
		result bool
	}{
		// Success
		{"v2s2.manifest.json", TestDockerV2S2ManifestDigest, true},
		{"v2s1.manifest.json", TestDockerV2S1ManifestDigest, true},
		// No match (switched s1/s2)
		{"v2s2.manifest.json", TestDockerV2S1ManifestDigest, false},
		{"v2s1.manifest.json", TestDockerV2S2ManifestDigest, false},
		// Unrecognized algorithm
		{"v2s2.manifest.json", "md5:2872f31c5c1f62a694fbd20c1e85257c", false},
		// Mangled format
		{"v2s2.manifest.json", TestDockerV2S2ManifestDigest + "abc", false},
		{"v2s2.manifest.json", TestDockerV2S2ManifestDigest[:20], false},
		{"v2s2.manifest.json", "", false},
	}
	for _, c := range cases {
		manifest, err := ioutil.ReadFile(filepath.Join("fixtures", c.path))
		require.NoError(t, err)
		res, err := MatchesDigest(manifest, c.digest)
		require.NoError(t, err)
		assert.Equal(t, c.result, res)
	}

	manifest, err := ioutil.ReadFile("fixtures/v2s1-invalid-signatures.manifest.json")
	require.NoError(t, err)
	// Even a correct SHA256 hash is rejected if we can't strip the JSON signature.
	hash := sha256.Sum256(manifest)
	res, err := MatchesDigest(manifest, "sha256:"+hex.EncodeToString(hash[:]))
	assert.False(t, res)
	assert.Error(t, err)

	res, err = MatchesDigest([]byte{}, "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	assert.True(t, res)
	assert.NoError(t, err)
}
