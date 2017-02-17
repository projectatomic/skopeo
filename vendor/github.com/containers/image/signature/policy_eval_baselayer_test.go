package signature

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPRSignedBaseLayerIsSignatureAuthorAccepted(t *testing.T) {
	pr, err := NewPRSignedBaseLayer(NewPRMMatchRepository())
	require.NoError(t, err)
	// Pass nil pointers to, kind of, test that the return value does not depend on the parameters.
	sar, parsedSig, err := pr.isSignatureAuthorAccepted(nil, nil)
	assertSARUnknown(t, sar, parsedSig, err)
}

func TestPRSignedBaseLayerIsRunningImageAllowed(t *testing.T) {
	// This will obviously need to change after signedBaseLayer is implemented.
	pr, err := NewPRSignedBaseLayer(NewPRMMatchRepository())
	require.NoError(t, err)
	// Pass a nil pointer to, kind of, test that the return value does not depend on the image.
	res, err := pr.isRunningImageAllowed(nil)
	assertRunningRejectedPolicyRequirement(t, res, err)
}
