package signature

import (
	"fmt"
	"os"
	"testing"

	"github.com/containers/image/docker/policyconfiguration"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyRequirementError(t *testing.T) {
	// A stupid test just to keep code coverage
	s := "test"
	err := PolicyRequirementError(s)
	assert.Equal(t, s, err.Error())
}

func TestPolicyContextChangeState(t *testing.T) {
	pc, err := NewPolicyContext(&Policy{Default: PolicyRequirements{NewPRReject()}})
	require.NoError(t, err)
	defer pc.Destroy()

	require.Equal(t, pcReady, pc.state)
	err = pc.changeState(pcReady, pcInUse)
	require.NoError(t, err)

	err = pc.changeState(pcReady, pcInUse)
	require.Error(t, err)

	// Return state to pcReady to allow pc.Destroy to clean up.
	err = pc.changeState(pcInUse, pcReady)
	require.NoError(t, err)
}

func TestPolicyContextNewDestroy(t *testing.T) {
	pc, err := NewPolicyContext(&Policy{Default: PolicyRequirements{NewPRReject()}})
	require.NoError(t, err)
	assert.Equal(t, pcReady, pc.state)

	err = pc.Destroy()
	require.NoError(t, err)
	assert.Equal(t, pcDestroyed, pc.state)

	// Trying to destroy when not pcReady
	pc, err = NewPolicyContext(&Policy{Default: PolicyRequirements{NewPRReject()}})
	require.NoError(t, err)
	err = pc.changeState(pcReady, pcInUse)
	require.NoError(t, err)
	err = pc.Destroy()
	require.Error(t, err)
	assert.Equal(t, pcInUse, pc.state) // The state, and hopefully nothing else, has changed.

	err = pc.changeState(pcInUse, pcReady)
	require.NoError(t, err)
	err = pc.Destroy()
	assert.NoError(t, err)
}

// pcImageReferenceMock is a mock of types.ImageReference which returns itself in DockerReference
// and handles PolicyConfigurationIdentity and PolicyConfigurationReference consistently.
type pcImageReferenceMock struct {
	transportName string
	ref           reference.Named
}

func (ref pcImageReferenceMock) Transport() types.ImageTransport {
	return nameImageTransportMock(ref.transportName)
}
func (ref pcImageReferenceMock) StringWithinTransport() string {
	// We use this in error messages, so sadly we must return something.
	return "== StringWithinTransport mock"
}
func (ref pcImageReferenceMock) DockerReference() reference.Named {
	return ref.ref
}
func (ref pcImageReferenceMock) PolicyConfigurationIdentity() string {
	res, err := policyconfiguration.DockerReferenceIdentity(ref.ref)
	if res == "" || err != nil {
		panic(fmt.Sprintf("Internal inconsistency: policyconfiguration.DockerReferenceIdentity returned %#v, %v", res, err))
	}
	return res
}
func (ref pcImageReferenceMock) PolicyConfigurationNamespaces() []string {
	if ref.ref == nil {
		panic("unexpected call to a mock function")
	}
	return policyconfiguration.DockerReferenceNamespaces(ref.ref)
}
func (ref pcImageReferenceMock) NewImage(ctx *types.SystemContext) (types.Image, error) {
	panic("unexpected call to a mock function")
}
func (ref pcImageReferenceMock) NewImageSource(ctx *types.SystemContext, requestedManifestMIMETypes []string) (types.ImageSource, error) {
	panic("unexpected call to a mock function")
}
func (ref pcImageReferenceMock) NewImageDestination(ctx *types.SystemContext) (types.ImageDestination, error) {
	panic("unexpected call to a mock function")
}
func (ref pcImageReferenceMock) DeleteImage(ctx *types.SystemContext) error {
	panic("unexpected call to a mock function")
}

func TestPolicyContextRequirementsForImageRef(t *testing.T) {
	ktGPG := SBKeyTypeGPGKeys
	prm := NewPRMMatchRepoDigestOrExact()

	policy := &Policy{
		Default:    PolicyRequirements{NewPRReject()},
		Transports: map[string]PolicyTransportScopes{},
	}
	// Just put _something_ into the PolicyTransportScopes map for the keys we care about, and make it pairwise
	// distinct so that we can compare the values and show them when debugging the tests.
	for _, t := range []struct{ transport, scope string }{
		{"docker", ""},
		{"docker", "unmatched"},
		{"docker", "deep.com"},
		{"docker", "deep.com/n1"},
		{"docker", "deep.com/n1/n2"},
		{"docker", "deep.com/n1/n2/n3"},
		{"docker", "deep.com/n1/n2/n3/repo"},
		{"docker", "deep.com/n1/n2/n3/repo:tag2"},
		{"atomic", "unmatched"},
	} {
		if _, ok := policy.Transports[t.transport]; !ok {
			policy.Transports[t.transport] = PolicyTransportScopes{}
		}
		policy.Transports[t.transport][t.scope] = PolicyRequirements{xNewPRSignedByKeyData(ktGPG, []byte(t.transport+t.scope), prm)}
	}

	pc, err := NewPolicyContext(policy)
	require.NoError(t, err)

	for _, c := range []struct{ inputTransport, input, matchedTransport, matched string }{
		// Full match
		{"docker", "deep.com/n1/n2/n3/repo:tag2", "docker", "deep.com/n1/n2/n3/repo:tag2"},
		// Namespace matches
		{"docker", "deep.com/n1/n2/n3/repo:nottag2", "docker", "deep.com/n1/n2/n3/repo"},
		{"docker", "deep.com/n1/n2/n3/notrepo:tag2", "docker", "deep.com/n1/n2/n3"},
		{"docker", "deep.com/n1/n2/notn3/repo:tag2", "docker", "deep.com/n1/n2"},
		{"docker", "deep.com/n1/notn2/n3/repo:tag2", "docker", "deep.com/n1"},
		// Host name match
		{"docker", "deep.com/notn1/n2/n3/repo:tag2", "docker", "deep.com"},
		// Default
		{"docker", "this.doesnt/match:anything", "docker", ""},
		// No match within a matched transport which doesn't have a "" scope
		{"atomic", "this.doesnt/match:anything", "", ""},
		// No configuration available for this transport at all
		{"dir", "what/ever", "", ""}, // "what/ever" is not a valid scope for the real "dir" transport, but we only need it to be a valid reference.Named.
	} {
		var expected PolicyRequirements
		if c.matchedTransport != "" {
			e, ok := policy.Transports[c.matchedTransport][c.matched]
			require.True(t, ok, fmt.Sprintf("case %s:%s: expected reqs not found", c.inputTransport, c.input))
			expected = e
		} else {
			expected = policy.Default
		}

		ref, err := reference.ParseNormalizedNamed(c.input)
		require.NoError(t, err)
		reqs := pc.requirementsForImageRef(pcImageReferenceMock{c.inputTransport, ref})
		comment := fmt.Sprintf("case %s:%s: %#v", c.inputTransport, c.input, reqs[0])
		// Do not use assert.Equal, which would do a deep contents comparison; we want to compare
		// the pointers. Also, == does not work on slices; so test that the slices start at the
		// same element and have the same length.
		assert.True(t, &(reqs[0]) == &(expected[0]), comment)
		assert.True(t, len(reqs) == len(expected), comment)
	}
}

// pcImageMock returns a types.UnparsedImage for a directory, claiming a specified dockerReference and implementing PolicyConfigurationIdentity/PolicyConfigurationNamespaces.
// The caller must call .Close() on the returned Image.
func pcImageMock(t *testing.T, dir, dockerReference string) types.UnparsedImage {
	ref, err := reference.ParseNormalizedNamed(dockerReference)
	require.NoError(t, err)
	return dirImageMockWithRef(t, dir, pcImageReferenceMock{"docker", ref})
}

func TestPolicyContextGetSignaturesWithAcceptedAuthor(t *testing.T) {
	expectedSig := &Signature{
		DockerManifestDigest: TestImageManifestDigest,
		DockerReference:      "testing/manifest:latest",
	}

	pc, err := NewPolicyContext(&Policy{
		Default: PolicyRequirements{NewPRReject()},
		Transports: map[string]PolicyTransportScopes{
			"docker": {
				"docker.io/testing/manifest:latest": {
					xNewPRSignedByKeyPath(SBKeyTypeGPGKeys, "fixtures/public-key.gpg", NewPRMMatchExact()),
				},
				"docker.io/testing/manifest:twoAccepts": {
					xNewPRSignedByKeyPath(SBKeyTypeGPGKeys, "fixtures/public-key.gpg", NewPRMMatchRepository()),
					xNewPRSignedByKeyPath(SBKeyTypeGPGKeys, "fixtures/public-key.gpg", NewPRMMatchRepository()),
				},
				"docker.io/testing/manifest:acceptReject": {
					xNewPRSignedByKeyPath(SBKeyTypeGPGKeys, "fixtures/public-key.gpg", NewPRMMatchRepository()),
					NewPRReject(),
				},
				"docker.io/testing/manifest:acceptUnknown": {
					xNewPRSignedByKeyPath(SBKeyTypeGPGKeys, "fixtures/public-key.gpg", NewPRMMatchRepository()),
					xNewPRSignedBaseLayer(NewPRMMatchRepository()),
				},
				"docker.io/testing/manifest:rejectUnknown": {
					NewPRReject(),
					xNewPRSignedBaseLayer(NewPRMMatchRepository()),
				},
				"docker.io/testing/manifest:unknown": {
					xNewPRSignedBaseLayer(NewPRMMatchRepository()),
				},
				"docker.io/testing/manifest:unknown2": {
					NewPRInsecureAcceptAnything(),
				},
				"docker.io/testing/manifest:invalidEmptyRequirements": {},
			},
		},
	})
	require.NoError(t, err)
	defer pc.Destroy()

	// Success
	img := pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:latest")
	defer img.Close()
	sigs, err := pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Equal(t, []*Signature{expectedSig}, sigs)

	// Two signatures
	// FIXME? Use really different signatures for this?
	img = pcImageMock(t, "fixtures/dir-img-valid-2", "testing/manifest:latest")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Equal(t, []*Signature{expectedSig, expectedSig}, sigs)

	// No signatures
	img = pcImageMock(t, "fixtures/dir-img-unsigned", "testing/manifest:latest")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Empty(t, sigs)

	// Only invalid signatures
	img = pcImageMock(t, "fixtures/dir-img-modified-manifest", "testing/manifest:latest")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Empty(t, sigs)

	// 1 invalid, 1 valid signature (in this order)
	img = pcImageMock(t, "fixtures/dir-img-mixed", "testing/manifest:latest")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Equal(t, []*Signature{expectedSig}, sigs)

	// Two sarAccepted results for one signature
	img = pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:twoAccepts")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Equal(t, []*Signature{expectedSig}, sigs)

	// sarAccepted+sarRejected for a signature
	img = pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:acceptReject")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Empty(t, sigs)

	// sarAccepted+sarUnknown for a signature
	img = pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:acceptUnknown")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Equal(t, []*Signature{expectedSig}, sigs)

	// sarRejected+sarUnknown for a signature
	img = pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:rejectUnknown")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Empty(t, sigs)

	// sarUnknown only
	img = pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:unknown")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Empty(t, sigs)

	img = pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:unknown2")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Empty(t, sigs)

	// Empty list of requirements (invalid)
	img = pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:invalidEmptyRequirements")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	require.NoError(t, err)
	assert.Empty(t, sigs)

	// Failures: Make sure we return nil sigs.

	// Unexpected state (context already destroyed)
	destroyedPC, err := NewPolicyContext(pc.Policy)
	require.NoError(t, err)
	err = destroyedPC.Destroy()
	require.NoError(t, err)
	img = pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:latest")
	defer img.Close()
	sigs, err = destroyedPC.GetSignaturesWithAcceptedAuthor(img)
	assert.Error(t, err)
	assert.Nil(t, sigs)
	// Not testing the pcInUse->pcReady transition, that would require custom PolicyRequirement
	// implementations meddling with the state, or threads. This is for catching trivial programmer
	// mistakes only, anyway.

	// Error reading signatures.
	invalidSigDir := createInvalidSigDir(t)
	defer os.RemoveAll(invalidSigDir)
	img = pcImageMock(t, invalidSigDir, "testing/manifest:latest")
	defer img.Close()
	sigs, err = pc.GetSignaturesWithAcceptedAuthor(img)
	assert.Error(t, err)
	assert.Nil(t, sigs)
}

func TestPolicyContextIsRunningImageAllowed(t *testing.T) {
	pc, err := NewPolicyContext(&Policy{
		Default: PolicyRequirements{NewPRReject()},
		Transports: map[string]PolicyTransportScopes{
			"docker": {
				"docker.io/testing/manifest:latest": {
					xNewPRSignedByKeyPath(SBKeyTypeGPGKeys, "fixtures/public-key.gpg", NewPRMMatchExact()),
				},
				"docker.io/testing/manifest:twoAllows": {
					xNewPRSignedByKeyPath(SBKeyTypeGPGKeys, "fixtures/public-key.gpg", NewPRMMatchRepository()),
					xNewPRSignedByKeyPath(SBKeyTypeGPGKeys, "fixtures/public-key.gpg", NewPRMMatchRepository()),
				},
				"docker.io/testing/manifest:allowDeny": {
					xNewPRSignedByKeyPath(SBKeyTypeGPGKeys, "fixtures/public-key.gpg", NewPRMMatchRepository()),
					NewPRReject(),
				},
				"docker.io/testing/manifest:reject": {
					NewPRReject(),
				},
				"docker.io/testing/manifest:acceptAnything": {
					NewPRInsecureAcceptAnything(),
				},
				"docker.io/testing/manifest:invalidEmptyRequirements": {},
			},
		},
	})
	require.NoError(t, err)
	defer pc.Destroy()

	// Success
	img := pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:latest")
	defer img.Close()
	res, err := pc.IsRunningImageAllowed(img)
	assertRunningAllowed(t, res, err)

	// Two signatures
	// FIXME? Use really different signatures for this?
	img = pcImageMock(t, "fixtures/dir-img-valid-2", "testing/manifest:latest")
	defer img.Close()
	res, err = pc.IsRunningImageAllowed(img)
	assertRunningAllowed(t, res, err)

	// No signatures
	img = pcImageMock(t, "fixtures/dir-img-unsigned", "testing/manifest:latest")
	defer img.Close()
	res, err = pc.IsRunningImageAllowed(img)
	assertRunningRejectedPolicyRequirement(t, res, err)

	// Only invalid signatures
	img = pcImageMock(t, "fixtures/dir-img-modified-manifest", "testing/manifest:latest")
	defer img.Close()
	res, err = pc.IsRunningImageAllowed(img)
	assertRunningRejectedPolicyRequirement(t, res, err)

	// 1 invalid, 1 valid signature (in this order)
	img = pcImageMock(t, "fixtures/dir-img-mixed", "testing/manifest:latest")
	defer img.Close()
	res, err = pc.IsRunningImageAllowed(img)
	assertRunningAllowed(t, res, err)

	// Two allowed results
	img = pcImageMock(t, "fixtures/dir-img-mixed", "testing/manifest:twoAllows")
	defer img.Close()
	res, err = pc.IsRunningImageAllowed(img)
	assertRunningAllowed(t, res, err)

	// Allow + deny results
	img = pcImageMock(t, "fixtures/dir-img-mixed", "testing/manifest:allowDeny")
	defer img.Close()
	res, err = pc.IsRunningImageAllowed(img)
	assertRunningRejectedPolicyRequirement(t, res, err)

	// prReject works
	img = pcImageMock(t, "fixtures/dir-img-mixed", "testing/manifest:reject")
	defer img.Close()
	res, err = pc.IsRunningImageAllowed(img)
	assertRunningRejectedPolicyRequirement(t, res, err)

	// prInsecureAcceptAnything works
	img = pcImageMock(t, "fixtures/dir-img-mixed", "testing/manifest:acceptAnything")
	defer img.Close()
	res, err = pc.IsRunningImageAllowed(img)
	assertRunningAllowed(t, res, err)

	// Empty list of requirements (invalid)
	img = pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:invalidEmptyRequirements")
	defer img.Close()
	res, err = pc.IsRunningImageAllowed(img)
	assertRunningRejectedPolicyRequirement(t, res, err)

	// Unexpected state (context already destroyed)
	destroyedPC, err := NewPolicyContext(pc.Policy)
	require.NoError(t, err)
	err = destroyedPC.Destroy()
	require.NoError(t, err)
	img = pcImageMock(t, "fixtures/dir-img-valid", "testing/manifest:latest")
	defer img.Close()
	res, err = destroyedPC.IsRunningImageAllowed(img)
	assertRunningRejected(t, res, err)
	// Not testing the pcInUse->pcReady transition, that would require custom PolicyRequirement
	// implementations meddling with the state, or threads. This is for catching trivial programmer
	// mistakes only, anyway.
}

// Helpers for validating PolicyRequirement.isSignatureAuthorAccepted results:

// assertSARRejected verifies that isSignatureAuthorAccepted returns a consistent sarRejected result
// with the expected signature.
func assertSARAccepted(t *testing.T, sar signatureAcceptanceResult, parsedSig *Signature, err error, expectedSig Signature) {
	assert.Equal(t, sarAccepted, sar)
	assert.Equal(t, &expectedSig, parsedSig)
	assert.NoError(t, err)
}

// assertSARRejected verifies that isSignatureAuthorAccepted returns a consistent sarRejected result.
func assertSARRejected(t *testing.T, sar signatureAcceptanceResult, parsedSig *Signature, err error) {
	assert.Equal(t, sarRejected, sar)
	assert.Nil(t, parsedSig)
	assert.Error(t, err)
}

// assertSARRejectedPolicyRequiremnt verifies that isSignatureAuthorAccepted returns a consistent sarRejected resul,
// and that the returned error is a PolicyRequirementError..
func assertSARRejectedPolicyRequirement(t *testing.T, sar signatureAcceptanceResult, parsedSig *Signature, err error) {
	assertSARRejected(t, sar, parsedSig, err)
	assert.IsType(t, PolicyRequirementError(""), err)
}

// assertSARRejected verifies that isSignatureAuthorAccepted returns a consistent sarUnknown result.
func assertSARUnknown(t *testing.T, sar signatureAcceptanceResult, parsedSig *Signature, err error) {
	assert.Equal(t, sarUnknown, sar)
	assert.Nil(t, parsedSig)
	assert.NoError(t, err)
}

// Helpers for validating PolicyRequirement.isRunningImageAllowed results:

// assertRunningAllowed verifies that isRunningImageAllowed returns a consistent true result
func assertRunningAllowed(t *testing.T, allowed bool, err error) {
	assert.Equal(t, true, allowed)
	assert.NoError(t, err)
}

// assertRunningRejected verifies that isRunningImageAllowed returns a consistent false result
func assertRunningRejected(t *testing.T, allowed bool, err error) {
	assert.Equal(t, false, allowed)
	assert.Error(t, err)
}

// assertRunningRejectedPolicyRequirement verifies that isRunningImageAllowed returns a consistent false result
// and that the returned error is a PolicyRequirementError.
func assertRunningRejectedPolicyRequirement(t *testing.T, allowed bool, err error) {
	assertRunningRejected(t, allowed, err)
	assert.IsType(t, PolicyRequirementError(""), err)
}
