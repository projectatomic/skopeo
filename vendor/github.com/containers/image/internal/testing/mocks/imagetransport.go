package mocks

import "github.com/containers/image/types"

// NameImageTransport is a mock of types.ImageTransport which returns itself in Name.
type NameImageTransport string

// Name returns the name of the transport, which must be unique among other transports.
func (name NameImageTransport) Name() string {
	return string(name)
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an ImageReference.
func (name NameImageTransport) ParseReference(reference string) (types.ImageReference, error) {
	panic("unexpected call to a mock function")
}

// ValidatePolicyConfigurationScope checks that scope is a valid name for a signature.PolicyTransportScopes keys
// (i.e. a valid PolicyConfigurationIdentity() or PolicyConfigurationNamespaces() return value).
// It is acceptable to allow an invalid value which will never be matched, it can "only" cause user confusion.
// scope passed to this function will not be "", that value is always allowed.
func (name NameImageTransport) ValidatePolicyConfigurationScope(scope string) error {
	panic("unexpected call to a mock function")
}
