package daemon

import (
	"testing"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sha256digestHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	sha256digest    = "sha256:" + sha256digestHex
)

func TestTransportName(t *testing.T) {
	assert.Equal(t, "docker-daemon", Transport.Name())
}

func TestTransportParseReference(t *testing.T) {
	testParseReference(t, Transport.ParseReference)
}

func TestTransportValidatePolicyConfigurationScope(t *testing.T) {
	for _, scope := range []string{ // A semi-representative assortment of values; everything is rejected.
		sha256digestHex,
		sha256digest,
		"docker.io/library/busybox:latest",
		"docker.io",
		"",
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.Error(t, err, scope)
	}
}

func TestParseReference(t *testing.T) {
	testParseReference(t, ParseReference)
}

// testParseReference is a test shared for Transport.ParseReference and ParseReference.
func testParseReference(t *testing.T, fn func(string) (types.ImageReference, error)) {
	for _, c := range []struct{ input, expectedID, expectedRef string }{
		{sha256digest, sha256digest, ""},                                                    // Valid digest format
		{"sha512:" + sha256digestHex + sha256digestHex, "", ""},                             // Non-digest.Canonical digest
		{"sha256:ab", "", ""},                                                               // Invalid digest value (too short)
		{sha256digest + "ab", "", ""},                                                       // Invalid digest value (too long)
		{"sha256:XX23456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "", ""}, // Invalid digest value
		{"UPPERCASEISINVALID", "", ""},                                                      // Invalid reference input
		{"busybox", "", ""},                                                                 // Missing tag or digest
		{"busybox:latest", "", "docker.io/library/busybox:latest"},                          // Explicit tag
		{"busybox@" + sha256digest, "", "docker.io/library/busybox@" + sha256digest},        // Explicit digest
		// A github.com/distribution/reference value can have a tag and a digest at the same time!
		// Most versions of docker/reference do not handle that (ignoring the tag), so we reject such input.
		{"busybox:latest@" + sha256digest, "", ""},                                   // Both tag and digest
		{"docker.io/library/busybox:latest", "", "docker.io/library/busybox:latest"}, // All implied values explicitly specified
	} {
		ref, err := fn(c.input)
		if c.expectedID == "" && c.expectedRef == "" {
			assert.Error(t, err, c.input)
		} else {
			require.NoError(t, err, c.input)
			daemonRef, ok := ref.(daemonReference)
			require.True(t, ok, c.input)
			// If we don't reject the input, the interpretation must be consistent with reference.ParseAnyReference
			dockerRef, err := reference.ParseAnyReference(c.input)
			require.NoError(t, err, c.input)

			if c.expectedRef == "" {
				assert.Equal(t, c.expectedID, daemonRef.id.String(), c.input)
				assert.Nil(t, daemonRef.ref, c.input)

				_, ok := dockerRef.(reference.Digested)
				require.True(t, ok, c.input)
				assert.Equal(t, c.expectedID, dockerRef.String(), c.input)
			} else {
				assert.Equal(t, "", daemonRef.id.String(), c.input)
				require.NotNil(t, daemonRef.ref, c.input)
				assert.Equal(t, c.expectedRef, daemonRef.ref.String(), c.input)

				_, ok := dockerRef.(reference.Named)
				require.True(t, ok, c.input)
				assert.Equal(t, c.expectedRef, dockerRef.String(), c.input)
			}
		}
	}
}

// A common list of reference formats to test for the various ImageReference methods.
// (For IDs it is much simpler, we simply use them unmodified)
var validNamedReferenceTestCases = []struct{ input, dockerRef, stringWithinTransport string }{
	{"busybox:notlatest", "docker.io/library/busybox:notlatest", "busybox:notlatest"},                // Explicit tag
	{"busybox" + sha256digest, "docker.io/library/busybox" + sha256digest, "busybox" + sha256digest}, // Explicit digest
	{"docker.io/library/busybox:latest", "docker.io/library/busybox:latest", "busybox:latest"},       // All implied values explicitly specified
	{"example.com/ns/foo:bar", "example.com/ns/foo:bar", "example.com/ns/foo:bar"},                   // All values explicitly specified
}

func TestNewReference(t *testing.T) {
	// An ID reference.
	id, err := digest.Parse(sha256digest)
	require.NoError(t, err)
	ref, err := NewReference(id, nil)
	require.NoError(t, err)
	daemonRef, ok := ref.(daemonReference)
	require.True(t, ok)
	assert.Equal(t, id, daemonRef.id)
	assert.Nil(t, daemonRef.ref)

	// Named references
	for _, c := range validNamedReferenceTestCases {
		parsed, err := reference.ParseNormalizedNamed(c.input)
		require.NoError(t, err)
		ref, err := NewReference("", parsed)
		require.NoError(t, err, c.input)
		daemonRef, ok := ref.(daemonReference)
		require.True(t, ok, c.input)
		assert.Equal(t, "", daemonRef.id.String())
		require.NotNil(t, daemonRef.ref)
		assert.Equal(t, c.dockerRef, daemonRef.ref.String(), c.input)
	}

	// Both an ID and a named reference provided
	parsed, err := reference.ParseNormalizedNamed("busybox:latest")
	require.NoError(t, err)
	_, err = NewReference(id, parsed)
	assert.Error(t, err)

	// A reference with neither a tag nor digest
	parsed, err = reference.ParseNormalizedNamed("busybox")
	require.NoError(t, err)
	_, err = NewReference("", parsed)
	assert.Error(t, err)

	// A github.com/distribution/reference value can have a tag and a digest at the same time!
	parsed, err = reference.ParseNormalizedNamed("busybox:notlatest@" + sha256digest)
	require.NoError(t, err)
	_, ok = parsed.(reference.Canonical)
	require.True(t, ok)
	_, ok = parsed.(reference.NamedTagged)
	require.True(t, ok)
	_, err = NewReference("", parsed)
	assert.Error(t, err)
}

func TestReferenceTransport(t *testing.T) {
	ref, err := ParseReference(sha256digest)
	require.NoError(t, err)
	assert.Equal(t, Transport, ref.Transport())

	ref, err = ParseReference("busybox:latest")
	require.NoError(t, err)
	assert.Equal(t, Transport, ref.Transport())
}

func TestReferenceStringWithinTransport(t *testing.T) {
	ref, err := ParseReference(sha256digest)
	require.NoError(t, err)
	assert.Equal(t, sha256digest, ref.StringWithinTransport())

	for _, c := range validNamedReferenceTestCases {
		ref, err := ParseReference(c.input)
		require.NoError(t, err, c.input)
		stringRef := ref.StringWithinTransport()
		assert.Equal(t, c.stringWithinTransport, stringRef, c.input)
		// Do one more round to verify that the output can be parsed, to an equal value.
		ref2, err := Transport.ParseReference(stringRef)
		require.NoError(t, err, c.input)
		stringRef2 := ref2.StringWithinTransport()
		assert.Equal(t, stringRef, stringRef2, c.input)
	}
}

func TestReferenceDockerReference(t *testing.T) {
	ref, err := ParseReference(sha256digest)
	require.NoError(t, err)
	assert.Nil(t, ref.DockerReference())

	for _, c := range validNamedReferenceTestCases {
		ref, err := ParseReference(c.input)
		require.NoError(t, err, c.input)
		dockerRef := ref.DockerReference()
		require.NotNil(t, dockerRef, c.input)
		assert.Equal(t, c.dockerRef, dockerRef.String(), c.input)
	}
}

func TestReferencePolicyConfigurationIdentity(t *testing.T) {
	ref, err := ParseReference(sha256digest)
	require.NoError(t, err)
	assert.Equal(t, "", ref.PolicyConfigurationIdentity())

	for _, c := range validNamedReferenceTestCases {
		ref, err := ParseReference(c.input)
		require.NoError(t, err, c.input)
		assert.Equal(t, "", ref.PolicyConfigurationIdentity(), c.input)
	}
}

func TestReferencePolicyConfigurationNamespaces(t *testing.T) {
	ref, err := ParseReference(sha256digest)
	require.NoError(t, err)
	assert.Empty(t, ref.PolicyConfigurationNamespaces())

	for _, c := range validNamedReferenceTestCases {
		ref, err := ParseReference(c.input)
		require.NoError(t, err, c.input)
		assert.Empty(t, ref.PolicyConfigurationNamespaces(), c.input)
	}
}

// daemonReference.NewImage, daemonReference.NewImageSource, openshiftReference.NewImageDestination
// untested because just creating the objects immediately connects to the daemon.

func TestReferenceDeleteImage(t *testing.T) {
	ref, err := ParseReference(sha256digest)
	require.NoError(t, err)
	err = ref.DeleteImage(nil)
	assert.Error(t, err)

	for _, c := range validNamedReferenceTestCases {
		ref, err := ParseReference(c.input)
		require.NoError(t, err, c.input)
		err = ref.DeleteImage(nil)
		assert.Error(t, err, c.input)
	}
}
