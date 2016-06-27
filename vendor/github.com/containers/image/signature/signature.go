// Note: Consider the API unstable until the code supports at least three different image formats or transports.

package signature

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/containers/image/version"
)

const (
	signatureType = "atomic container signature"
)

// InvalidSignatureError is returned when parsing an invalid signature.
type InvalidSignatureError struct {
	msg string
}

func (err InvalidSignatureError) Error() string {
	return err.msg
}

// Signature is a parsed content of a signature.
type Signature struct {
	DockerManifestDigest string // FIXME: more precise type?
	DockerReference      string // FIXME: more precise type?
}

// Wrap signature to add to it some methods which we don't want to make public.
type privateSignature struct {
	Signature
}

// Compile-time check that privateSignature implements json.Marshaler
var _ json.Marshaler = (*privateSignature)(nil)

// MarshalJSON implements the json.Marshaler interface.
func (s privateSignature) MarshalJSON() ([]byte, error) {
	return s.marshalJSONWithVariables(time.Now().UTC().Unix(), "atomic "+version.Version)
}

// Implementation of MarshalJSON, with a caller-chosen values of the variable items to help testing.
func (s privateSignature) marshalJSONWithVariables(timestamp int64, creatorID string) ([]byte, error) {
	if s.DockerManifestDigest == "" || s.DockerReference == "" {
		return nil, errors.New("Unexpected empty signature content")
	}
	critical := map[string]interface{}{
		"type":     signatureType,
		"image":    map[string]string{"docker-manifest-digest": s.DockerManifestDigest},
		"identity": map[string]string{"docker-reference": s.DockerReference},
	}
	optional := map[string]interface{}{
		"creator":   creatorID,
		"timestamp": timestamp,
	}
	signature := map[string]interface{}{
		"critical": critical,
		"optional": optional,
	}
	return json.Marshal(signature)
}

// Compile-time check that privateSignature implements json.Unmarshaler
var _ json.Unmarshaler = (*privateSignature)(nil)

// UnmarshalJSON implements the json.Unmarshaler interface
func (s *privateSignature) UnmarshalJSON(data []byte) error {
	err := s.strictUnmarshalJSON(data)
	if err != nil {
		if _, ok := err.(jsonFormatError); ok {
			err = InvalidSignatureError{msg: err.Error()}
		}
	}
	return err
}

// strictUnmarshalJSON is UnmarshalJSON, except that it may return the internal jsonFormatError error type.
// Splitting it into a separate function allows us to do the jsonFormatError → InvalidSignatureError in a single place, the caller.
func (s *privateSignature) strictUnmarshalJSON(data []byte) error {
	var untyped interface{}
	if err := json.Unmarshal(data, &untyped); err != nil {
		return err
	}
	o, ok := untyped.(map[string]interface{})
	if !ok {
		return InvalidSignatureError{msg: "Invalid signature format"}
	}
	if err := validateExactMapKeys(o, "critical", "optional"); err != nil {
		return err
	}

	c, err := mapField(o, "critical")
	if err != nil {
		return err
	}
	if err := validateExactMapKeys(c, "type", "image", "identity"); err != nil {
		return err
	}

	optional, err := mapField(o, "optional")
	if err != nil {
		return err
	}
	_ = optional // We don't use anything from here for now.

	t, err := stringField(c, "type")
	if err != nil {
		return err
	}
	if t != signatureType {
		return InvalidSignatureError{msg: fmt.Sprintf("Unrecognized signature type %s", t)}
	}

	image, err := mapField(c, "image")
	if err != nil {
		return err
	}
	if err := validateExactMapKeys(image, "docker-manifest-digest"); err != nil {
		return err
	}
	digest, err := stringField(image, "docker-manifest-digest")
	if err != nil {
		return err
	}
	s.DockerManifestDigest = digest

	identity, err := mapField(c, "identity")
	if err != nil {
		return err
	}
	if err := validateExactMapKeys(identity, "docker-reference"); err != nil {
		return err
	}
	reference, err := stringField(identity, "docker-reference")
	if err != nil {
		return err
	}
	s.DockerReference = reference

	return nil
}

// Sign formats the signature and returns a blob signed using mech and keyIdentity
func (s privateSignature) sign(mech SigningMechanism, keyIdentity string) ([]byte, error) {
	json, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	return mech.Sign(json, keyIdentity)
}

// signatureAcceptanceRules specifies how to decide whether an untrusted signature is acceptable.
// We centralize the actual parsing and data extraction in verifyAndExtractSignature; this supplies
// the policy.  We use an object instead of supplying func parameters to verifyAndExtractSignature
// because all of the functions have the same type, so there is a risk of exchanging the functions;
// named members of this struct are more explicit.
type signatureAcceptanceRules struct {
	validateKeyIdentity                func(string) error
	validateSignedDockerReference      func(string) error
	validateSignedDockerManifestDigest func(string) error
}

// verifyAndExtractSignature verifies that unverifiedSignature has been signed, and that its principial components
// match expected values, both as specified by rules, and returns it
func verifyAndExtractSignature(mech SigningMechanism, unverifiedSignature []byte, rules signatureAcceptanceRules) (*Signature, error) {
	signed, keyIdentity, err := mech.Verify(unverifiedSignature)
	if err != nil {
		return nil, err
	}
	if err := rules.validateKeyIdentity(keyIdentity); err != nil {
		return nil, err
	}

	var unmatchedSignature privateSignature
	if err := json.Unmarshal(signed, &unmatchedSignature); err != nil {
		return nil, InvalidSignatureError{msg: err.Error()}
	}
	if err := rules.validateSignedDockerManifestDigest(unmatchedSignature.DockerManifestDigest); err != nil {
		return nil, err
	}
	if err := rules.validateSignedDockerReference(unmatchedSignature.DockerReference); err != nil {
		return nil, err
	}
	signature := unmatchedSignature.Signature // Policy OK.
	return &signature, nil
}
