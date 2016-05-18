package utils

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuessManifestMIMEType(t *testing.T) {
	cases := []struct {
		path     string
		mimeType string
	}{
		{"v2s2.manifest.json", V2Schema2MIMEType},
		{"v2list.manifest.json", V2ListMIMEType},
		{"v2s1.manifest.json", V2Schema1MIMEType},
		{"v2s1-invalid-signatures.manifest.json", V2Schema1MIMEType},
		{"v2s2nomime.manifest.json", V2Schema2MIMEType}, // It is unclear whether this one is legal, but we should guess v2s2 if anything at all.
		{"unknown-version.manifest.json", ""},
		{"non-json.manifest.json", ""}, // Not a manifest (nor JSON) at all
	}

	for _, c := range cases {
		manifest, err := ioutil.ReadFile(filepath.Join("fixtures", c.path))
		require.NoError(t, err)
		mimeType := GuessManifestMIMEType(manifest)
		assert.Equal(t, c.mimeType, mimeType)
	}
}

func TestManifestDigest(t *testing.T) {
	cases := []struct {
		path   string
		digest string
	}{
		{"v2s2.manifest.json", TestV2S2ManifestDigest},
		{"v2s1.manifest.json", TestV2S1ManifestDigest},
	}
	for _, c := range cases {
		manifest, err := ioutil.ReadFile(filepath.Join("fixtures", c.path))
		require.NoError(t, err)
		digest, err := ManifestDigest(manifest)
		require.NoError(t, err)
		assert.Equal(t, c.digest, digest)
	}

	manifest, err := ioutil.ReadFile("fixtures/v2s1-invalid-signatures.manifest.json")
	require.NoError(t, err)
	digest, err := ManifestDigest(manifest)
	assert.Error(t, err)

	digest, err = ManifestDigest([]byte{})
	require.NoError(t, err)
	assert.Equal(t, "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", digest)
}
