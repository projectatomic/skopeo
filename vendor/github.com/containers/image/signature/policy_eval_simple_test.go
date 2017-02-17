package signature

import (
	"testing"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
)

// nameOnlyImageMock is a mock of types.UnparsedImage which only allows transports.ImageName to work
type nameOnlyImageMock struct {
	forbiddenImageMock
}

func (nameOnlyImageMock) Reference() types.ImageReference {
	return nameOnlyImageReferenceMock("== StringWithinTransport mock")
}

// nameOnlyImageReferenceMock is a mock of types.ImageReference which only allows transports.ImageName to work, returning self.
type nameOnlyImageReferenceMock string

func (ref nameOnlyImageReferenceMock) Transport() types.ImageTransport {
	return nameImageTransportMock("== Transport mock")
}
func (ref nameOnlyImageReferenceMock) StringWithinTransport() string {
	return string(ref)
}
func (ref nameOnlyImageReferenceMock) DockerReference() reference.Named {
	panic("unexpected call to a mock function")
}
func (ref nameOnlyImageReferenceMock) PolicyConfigurationIdentity() string {
	panic("unexpected call to a mock function")
}
func (ref nameOnlyImageReferenceMock) PolicyConfigurationNamespaces() []string {
	panic("unexpected call to a mock function")
}
func (ref nameOnlyImageReferenceMock) NewImage(ctx *types.SystemContext) (types.Image, error) {
	panic("unexpected call to a mock function")
}
func (ref nameOnlyImageReferenceMock) NewImageSource(ctx *types.SystemContext, requestedManifestMIMETypes []string) (types.ImageSource, error) {
	panic("unexpected call to a mock function")
}
func (ref nameOnlyImageReferenceMock) NewImageDestination(ctx *types.SystemContext) (types.ImageDestination, error) {
	panic("unexpected call to a mock function")
}
func (ref nameOnlyImageReferenceMock) DeleteImage(ctx *types.SystemContext) error {
	panic("unexpected call to a mock function")
}

func TestPRInsecureAcceptAnythingIsSignatureAuthorAccepted(t *testing.T) {
	pr := NewPRInsecureAcceptAnything()
	// Pass nil signature to, kind of, test that the return value does not depend on it.
	sar, parsedSig, err := pr.isSignatureAuthorAccepted(nameOnlyImageMock{}, nil)
	assertSARUnknown(t, sar, parsedSig, err)
}

func TestPRInsecureAcceptAnythingIsRunningImageAllowed(t *testing.T) {
	pr := NewPRInsecureAcceptAnything()
	res, err := pr.isRunningImageAllowed(nameOnlyImageMock{})
	assertRunningAllowed(t, res, err)
}

func TestPRRejectIsSignatureAuthorAccepted(t *testing.T) {
	pr := NewPRReject()
	// Pass nil signature to, kind of, test that the return value does not depend on it.
	sar, parsedSig, err := pr.isSignatureAuthorAccepted(nameOnlyImageMock{}, nil)
	assertSARRejectedPolicyRequirement(t, sar, parsedSig, err)
}

func TestPRRejectIsRunningImageAllowed(t *testing.T) {
	pr := NewPRReject()
	res, err := pr.isRunningImageAllowed(nameOnlyImageMock{})
	assertRunningRejectedPolicyRequirement(t, res, err)
}
