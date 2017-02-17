package signature

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/containers/image/directory"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/image"
	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dirImageMock returns a types.UnparsedImage for a directory, claiming a specified dockerReference.
// The caller must call .Close() on the returned UnparsedImage.
func dirImageMock(t *testing.T, dir, dockerReference string) types.UnparsedImage {
	ref, err := reference.ParseNormalizedNamed(dockerReference)
	require.NoError(t, err)
	return dirImageMockWithRef(t, dir, refImageReferenceMock{ref})
}

// dirImageMockWithRef returns a types.UnparsedImage for a directory, claiming a specified ref.
// The caller must call .Close() on the returned UnparsedImage.
func dirImageMockWithRef(t *testing.T, dir string, ref types.ImageReference) types.UnparsedImage {
	srcRef, err := directory.NewReference(dir)
	require.NoError(t, err)
	src, err := srcRef.NewImageSource(nil, nil)
	require.NoError(t, err)
	return image.UnparsedFromSource(&dirImageSourceMock{
		ImageSource: src,
		ref:         ref,
	})
}

// dirImageSourceMock inherits dirImageSource, but overrides its Reference method.
type dirImageSourceMock struct {
	types.ImageSource
	ref types.ImageReference
}

func (d *dirImageSourceMock) Reference() types.ImageReference {
	return d.ref
}

func TestPRSignedByIsSignatureAuthorAccepted(t *testing.T) {
	ktGPG := SBKeyTypeGPGKeys
	prm := NewPRMMatchExact()
	testImage := dirImageMock(t, "fixtures/dir-img-valid", "testing/manifest:latest")
	defer testImage.Close()
	testImageSig, err := ioutil.ReadFile("fixtures/dir-img-valid/signature-1")
	require.NoError(t, err)

	// Successful validation, with KeyData and KeyPath
	pr, err := NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	sar, parsedSig, err := pr.isSignatureAuthorAccepted(testImage, testImageSig)
	assertSARAccepted(t, sar, parsedSig, err, Signature{
		DockerManifestDigest: TestImageManifestDigest,
		DockerReference:      "testing/manifest:latest",
	})

	keyData, err := ioutil.ReadFile("fixtures/public-key.gpg")
	require.NoError(t, err)
	pr, err = NewPRSignedByKeyData(ktGPG, keyData, prm)
	require.NoError(t, err)
	sar, parsedSig, err = pr.isSignatureAuthorAccepted(testImage, testImageSig)
	assertSARAccepted(t, sar, parsedSig, err, Signature{
		DockerManifestDigest: TestImageManifestDigest,
		DockerReference:      "testing/manifest:latest",
	})

	// Unimplemented and invalid KeyType values
	for _, keyType := range []sbKeyType{SBKeyTypeSignedByGPGKeys,
		SBKeyTypeX509Certificates,
		SBKeyTypeSignedByX509CAs,
		sbKeyType("This is invalid"),
	} {
		// Do not use NewPRSignedByKeyData, because it would reject invalid values.
		pr := &prSignedBy{
			KeyType:        keyType,
			KeyData:        []byte("abc"),
			SignedIdentity: prm,
		}
		// Pass nil pointers to, kind of, test that the return value does not depend on the parameters.
		sar, parsedSig, err := pr.isSignatureAuthorAccepted(nil, nil)
		assertSARRejected(t, sar, parsedSig, err)
	}

	// Both KeyPath and KeyData set. Do not use NewPRSignedBy*, because it would reject this.
	prSB := &prSignedBy{
		KeyType:        ktGPG,
		KeyPath:        "/foo/bar",
		KeyData:        []byte("abc"),
		SignedIdentity: prm,
	}
	// Pass nil pointers to, kind of, test that the return value does not depend on the parameters.
	sar, parsedSig, err = prSB.isSignatureAuthorAccepted(nil, nil)
	assertSARRejected(t, sar, parsedSig, err)

	// Invalid KeyPath
	pr, err = NewPRSignedByKeyPath(ktGPG, "/this/does/not/exist", prm)
	require.NoError(t, err)
	// Pass nil pointers to, kind of, test that the return value does not depend on the parameters.
	sar, parsedSig, err = pr.isSignatureAuthorAccepted(nil, nil)
	assertSARRejected(t, sar, parsedSig, err)

	// Errors initializing the temporary GPG directory and mechanism are not obviously easy to reach.

	// KeyData has no public keys.
	pr, err = NewPRSignedByKeyData(ktGPG, []byte{}, prm)
	require.NoError(t, err)
	// Pass nil pointers to, kind of, test that the return value does not depend on the parameters.
	sar, parsedSig, err = pr.isSignatureAuthorAccepted(nil, nil)
	assertSARRejectedPolicyRequirement(t, sar, parsedSig, err)

	// A signature which does not GPG verify
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	// Pass a nil pointer to, kind of, test that the return value does not depend on the image parmater..
	sar, parsedSig, err = pr.isSignatureAuthorAccepted(nil, []byte("invalid signature"))
	assertSARRejected(t, sar, parsedSig, err)

	// A valid signature using an unknown key.
	// (This is (currently?) rejected through the "mech.Verify fails" path, not the "!identityFound" path,
	// because we use a temporary directory and only import the trusted keys.)
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	sig, err := ioutil.ReadFile("fixtures/unknown-key.signature")
	require.NoError(t, err)
	// Pass a nil pointer to, kind of, test that the return value does not depend on the image parmater..
	sar, parsedSig, err = pr.isSignatureAuthorAccepted(nil, sig)
	assertSARRejected(t, sar, parsedSig, err)

	// A valid signature of an invalid JSON.
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	sig, err = ioutil.ReadFile("fixtures/invalid-blob.signature")
	require.NoError(t, err)
	// Pass a nil pointer to, kind of, test that the return value does not depend on the image parmater..
	sar, parsedSig, err = pr.isSignatureAuthorAccepted(nil, sig)
	assertSARRejected(t, sar, parsedSig, err)
	assert.IsType(t, InvalidSignatureError{}, err)

	// A valid signature with a rejected identity.
	nonmatchingPRM, err := NewPRMExactReference("this/doesnt:match")
	require.NoError(t, err)
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", nonmatchingPRM)
	require.NoError(t, err)
	sar, parsedSig, err = pr.isSignatureAuthorAccepted(testImage, testImageSig)
	assertSARRejectedPolicyRequirement(t, sar, parsedSig, err)

	// Error reading image manifest
	image := dirImageMock(t, "fixtures/dir-img-no-manifest", "testing/manifest:latest")
	defer image.Close()
	sig, err = ioutil.ReadFile("fixtures/dir-img-no-manifest/signature-1")
	require.NoError(t, err)
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	sar, parsedSig, err = pr.isSignatureAuthorAccepted(image, sig)
	assertSARRejected(t, sar, parsedSig, err)

	// Error computing manifest digest
	image = dirImageMock(t, "fixtures/dir-img-manifest-digest-error", "testing/manifest:latest")
	defer image.Close()
	sig, err = ioutil.ReadFile("fixtures/dir-img-manifest-digest-error/signature-1")
	require.NoError(t, err)
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	sar, parsedSig, err = pr.isSignatureAuthorAccepted(image, sig)
	assertSARRejected(t, sar, parsedSig, err)

	// A valid signature with a non-matching manifest
	image = dirImageMock(t, "fixtures/dir-img-modified-manifest", "testing/manifest:latest")
	defer image.Close()
	sig, err = ioutil.ReadFile("fixtures/dir-img-modified-manifest/signature-1")
	require.NoError(t, err)
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	sar, parsedSig, err = pr.isSignatureAuthorAccepted(image, sig)
	assertSARRejectedPolicyRequirement(t, sar, parsedSig, err)
}

// createInvalidSigDir creates a directory suitable for dirImageMock, in which image.Signatures()
// fails.
// The caller should eventually call os.RemoveAll on the returned path.
func createInvalidSigDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "skopeo-test-unreadable-signature")
	require.NoError(t, err)
	err = ioutil.WriteFile(path.Join(dir, "manifest.json"), []byte("{}"), 0644)
	require.NoError(t, err)
	// Creating a 000-permissions file would work for unprivileged accounts, but root (in particular,
	// in the Docker container we use for testing) would still have access.  So, create a symlink
	// pointing to itself, to cause an ELOOP. (Note that a symlink pointing to a nonexistent file would be treated
	// just like a nonexistent signature file, and not an error.)
	err = os.Symlink("signature-1", path.Join(dir, "signature-1"))
	require.NoError(t, err)
	return dir
}

func TestPRSignedByIsRunningImageAllowed(t *testing.T) {
	ktGPG := SBKeyTypeGPGKeys
	prm := NewPRMMatchExact()

	// A simple success case: single valid signature.
	image := dirImageMock(t, "fixtures/dir-img-valid", "testing/manifest:latest")
	defer image.Close()
	pr, err := NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	allowed, err := pr.isRunningImageAllowed(image)
	assertRunningAllowed(t, allowed, err)

	// Error reading signatures
	invalidSigDir := createInvalidSigDir(t)
	defer os.RemoveAll(invalidSigDir)
	image = dirImageMock(t, invalidSigDir, "testing/manifest:latest")
	defer image.Close()
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(image)
	assertRunningRejected(t, allowed, err)

	// No signatures
	image = dirImageMock(t, "fixtures/dir-img-unsigned", "testing/manifest:latest")
	defer image.Close()
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(image)
	assertRunningRejectedPolicyRequirement(t, allowed, err)

	// 1 invalid signature: use dir-img-valid, but a non-matching Docker reference
	image = dirImageMock(t, "fixtures/dir-img-valid", "testing/manifest:notlatest")
	defer image.Close()
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(image)
	assertRunningRejectedPolicyRequirement(t, allowed, err)

	// 2 valid signatures
	image = dirImageMock(t, "fixtures/dir-img-valid-2", "testing/manifest:latest")
	defer image.Close()
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(image)
	assertRunningAllowed(t, allowed, err)

	// One invalid, one valid signature (in this order)
	image = dirImageMock(t, "fixtures/dir-img-mixed", "testing/manifest:latest")
	defer image.Close()
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(image)
	assertRunningAllowed(t, allowed, err)

	// 2 invalid signatures: use dir-img-valid-2, but a non-matching Docker reference
	image = dirImageMock(t, "fixtures/dir-img-valid-2", "testing/manifest:notlatest")
	defer image.Close()
	pr, err = NewPRSignedByKeyPath(ktGPG, "fixtures/public-key.gpg", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(image)
	assertRunningRejectedPolicyRequirement(t, allowed, err)
}
