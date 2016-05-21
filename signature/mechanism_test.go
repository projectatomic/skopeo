package signature

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testGPGHomeDirectory = "./fixtures"
)

func TestNewGPGSigningMechanism(t *testing.T) {
	// A dumb test just for code coverage. We test more with newGPGSigningMechanismInDirectory().
	_, err := NewGPGSigningMechanism()
	assert.NoError(t, err)
}

func TestNewGPGSigningMechanismInDirectory(t *testing.T) {
	// A dumb test just for code coverage.
	_, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	assert.NoError(t, err)
	// The various GPG failure cases are not obviously easy to reach.
}

func TestGPGSigningMechanismSign(t *testing.T) {
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)

	// Successful signing
	content := []byte("content")
	signature, err := mech.Sign(content, TestKeyFingerprint)
	require.NoError(t, err)

	signedContent, signingFingerprint, err := mech.Verify(signature)
	require.NoError(t, err)
	assert.EqualValues(t, content, signedContent)
	assert.Equal(t, TestKeyFingerprint, signingFingerprint)

	// Error signing
	_, err = mech.Sign(content, "this fingerprint doesn't exist")
	assert.Error(t, err)
	// The various GPG/GPGME failures cases are not obviously easy to reach.
}

func assertSigningError(t *testing.T, content []byte, fingerprint string, err error) {
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Empty(t, fingerprint)
}

func TestGPGSigningMechanismVerify(t *testing.T) {
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)

	// Successful verification
	signature, err := ioutil.ReadFile("./fixtures/invalid-blob.signature")
	require.NoError(t, err)
	content, signingFingerprint, err := mech.Verify(signature)
	require.NoError(t, err)
	assert.Equal(t, []byte("This is not JSON\n"), content)
	assert.Equal(t, TestKeyFingerprint, signingFingerprint)

	// For extra paranoia, test that we return nil data on error.

	// Completely invalid signature.
	content, signingFingerprint, err = mech.Verify([]byte{})
	assertSigningError(t, content, signingFingerprint, err)

	content, signingFingerprint, err = mech.Verify([]byte("invalid signature"))
	assertSigningError(t, content, signingFingerprint, err)

	// Literal packet, not a signature
	signature, err = ioutil.ReadFile("./fixtures/unsigned-literal.signature")
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)

	// Encrypted data, not a signature.
	signature, err = ioutil.ReadFile("./fixtures/unsigned-encrypted.signature")
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)

	// FIXME? Is there a way to create a multi-signature so that gpgme_op_verify returns multiple signatures?

	// Expired signature
	signature, err = ioutil.ReadFile("./fixtures/expired.signature")
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)

	// Corrupt signature
	signature, err = ioutil.ReadFile("./fixtures/corrupt.signature")
	require.NoError(t, err)
	content, signingFingerprint, err = mech.Verify(signature)
	assertSigningError(t, content, signingFingerprint, err)
	// The various GPG/GPGME failures cases are not obviously easy to reach.
}
