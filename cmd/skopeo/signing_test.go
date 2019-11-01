package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/containers/image/v5/signature"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// fixturesTestImageManifestDigest is the Docker manifest digest of "image.manifest.json"
	fixturesTestImageManifestDigest = digest.Digest("sha256:20bf21ed457b390829cdbeec8795a7bea1626991fda603e0d01b4e7f60427e55")
	// fixturesTestKeyFingerprint is the fingerprint of the private key.
	fixturesTestKeyFingerprint = "1D8230F6CDB6A06716E414C1DB72F2188BB46CC8"
	// fixturesTestKeyFingerprint is the key ID of the private key.
	fixturesTestKeyShortID = "DB72F2188BB46CC8"
)

// Test that results of runSkopeo failed with nothing on stdout, and substring
// within the error message.
func assertTestFailed(t *testing.T, stdout string, err error, substring string) {
	assert.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, err.Error(), substring)
}

func TestStandaloneSign(t *testing.T) {
	mech, _, err := signature.NewEphemeralGPGSigningMechanism([]byte{})
	require.NoError(t, err)
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil {
		t.Skipf("Signing not supported: %v", err)
	}

	manifestPath := "fixtures/image.manifest.json"
	dockerReference := "testing/manifest"
	os.Setenv("GNUPGHOME", "fixtures")
	defer os.Unsetenv("GNUPGHOME")

	// Invalid command-line arguments
	for _, args := range [][]string{
		{},
		{"a1", "a2"},
		{"a1", "a2", "a3"},
		{"a1", "a2", "a3", "a4"},
		{"-o", "o", "a1", "a2"},
		{"-o", "o", "a1", "a2", "a3", "a4"},
	} {
		out, err := runSkopeo(append([]string{"standalone-sign"}, args...)...)
		assertTestFailed(t, out, err, "Usage")
	}

	// Error reading manifest
	out, err := runSkopeo("standalone-sign", "-o", "/dev/null",
		"/this/doesnt/exist", dockerReference, fixturesTestKeyFingerprint)
	assertTestFailed(t, out, err, "/this/doesnt/exist")

	// Invalid Docker reference
	out, err = runSkopeo("standalone-sign", "-o", "/dev/null",
		manifestPath, "" /* empty reference */, fixturesTestKeyFingerprint)
	assertTestFailed(t, out, err, "empty signature content")

	// Unknown key.
	out, err = runSkopeo("standalone-sign", "-o", "/dev/null",
		manifestPath, dockerReference, "UNKNOWN GPG FINGERPRINT")
	assert.Error(t, err)
	assert.Empty(t, out)

	// Error writing output
	out, err = runSkopeo("standalone-sign", "-o", "/dev/full",
		manifestPath, dockerReference, fixturesTestKeyFingerprint)
	assertTestFailed(t, out, err, "/dev/full")

	// Success
	sigOutput, err := ioutil.TempFile("", "sig")
	require.NoError(t, err)
	defer os.Remove(sigOutput.Name())
	out, err = runSkopeo("standalone-sign", "-o", sigOutput.Name(),
		manifestPath, dockerReference, fixturesTestKeyFingerprint)
	require.NoError(t, err)
	assert.Empty(t, out)

	sig, err := ioutil.ReadFile(sigOutput.Name())
	require.NoError(t, err)
	manifest, err := ioutil.ReadFile(manifestPath)
	require.NoError(t, err)
	mech, err = signature.NewGPGSigningMechanism()
	require.NoError(t, err)
	defer mech.Close()
	verified, err := signature.VerifyDockerManifestSignature(sig, manifest, dockerReference, mech, fixturesTestKeyFingerprint)
	require.NoError(t, err)
	assert.Equal(t, dockerReference, verified.DockerReference)
	assert.Equal(t, fixturesTestImageManifestDigest, verified.DockerManifestDigest)
}

func TestStandaloneVerify(t *testing.T) {
	manifestPath := "fixtures/image.manifest.json"
	signaturePath := "fixtures/image.signature"
	dockerReference := "testing/manifest"
	os.Setenv("GNUPGHOME", "fixtures")
	defer os.Unsetenv("GNUPGHOME")

	// Invalid command-line arguments
	for _, args := range [][]string{
		{},
		{"a1", "a2", "a3"},
		{"a1", "a2", "a3", "a4", "a5"},
	} {
		out, err := runSkopeo(append([]string{"standalone-verify"}, args...)...)
		assertTestFailed(t, out, err, "Usage")
	}

	// Error reading manifest
	out, err := runSkopeo("standalone-verify", "/this/doesnt/exist",
		dockerReference, fixturesTestKeyFingerprint, signaturePath)
	assertTestFailed(t, out, err, "/this/doesnt/exist")

	// Error reading signature
	out, err = runSkopeo("standalone-verify", manifestPath,
		dockerReference, fixturesTestKeyFingerprint, "/this/doesnt/exist")
	assertTestFailed(t, out, err, "/this/doesnt/exist")

	// Error verifying signature
	out, err = runSkopeo("standalone-verify", manifestPath,
		dockerReference, fixturesTestKeyFingerprint, "fixtures/corrupt.signature")
	assertTestFailed(t, out, err, "Error verifying signature")

	// Success
	out, err = runSkopeo("standalone-verify", manifestPath,
		dockerReference, fixturesTestKeyFingerprint, signaturePath)
	assert.NoError(t, err)
	assert.Equal(t, "Signature verified, digest "+fixturesTestImageManifestDigest.String()+"\n", out)
}

func TestUntrustedSignatureDump(t *testing.T) {
	// Invalid command-line arguments
	for _, args := range [][]string{
		{},
		{"a1", "a2"},
		{"a1", "a2", "a3", "a4"},
	} {
		out, err := runSkopeo(append([]string{"untrusted-signature-dump-without-verification"}, args...)...)
		assertTestFailed(t, out, err, "Usage")
	}

	// Error reading manifest
	out, err := runSkopeo("untrusted-signature-dump-without-verification",
		"/this/doesnt/exist")
	assertTestFailed(t, out, err, "/this/doesnt/exist")

	// Error reading signature (input is not a signature)
	out, err = runSkopeo("untrusted-signature-dump-without-verification", "fixtures/image.manifest.json")
	assertTestFailed(t, out, err, "Error decoding untrusted signature")

	// Success
	for _, path := range []string{"fixtures/image.signature", "fixtures/corrupt.signature"} {
		// Success
		out, err = runSkopeo("untrusted-signature-dump-without-verification", path)
		require.NoError(t, err)

		var info signature.UntrustedSignatureInformation
		err := json.Unmarshal([]byte(out), &info)
		require.NoError(t, err)
		assert.Equal(t, fixturesTestImageManifestDigest, info.UntrustedDockerManifestDigest)
		assert.Equal(t, "testing/manifest", info.UntrustedDockerReference)
		assert.NotNil(t, info.UntrustedCreatorID)
		assert.Equal(t, "atomic ", *info.UntrustedCreatorID)
		assert.NotNil(t, info.UntrustedTimestamp)
		assert.True(t, time.Unix(1458239713, 0).Equal(*info.UntrustedTimestamp))
		assert.Equal(t, fixturesTestKeyShortID, info.UntrustedShortKeyIdentifier)
	}
}
