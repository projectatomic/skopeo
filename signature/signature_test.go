package signature

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidSignatureError(t *testing.T) {
	// A stupid test just to keep code coverage
	s := "test"
	err := InvalidSignatureError{msg: s}
	assert.Equal(t, s, err.Error())
}

func TestMarshalJSON(t *testing.T) {
	// Empty string values
	s := privateSignature{Signature{DockerManifestDigest: "", DockerReference: "_"}}
	_, err := s.MarshalJSON()
	assert.Error(t, err)
	s = privateSignature{Signature{DockerManifestDigest: "_", DockerReference: ""}}
	_, err = s.MarshalJSON()
	assert.Error(t, err)

	// Success
	s = privateSignature{Signature{DockerManifestDigest: "digest!@#", DockerReference: "reference#@!"}}
	marshaled, err := s.marshalJSONWithVariables(0, "CREATOR")
	require.NoError(t, err)
	assert.Equal(t, []byte("{\"critical\":{\"identity\":{\"docker-reference\":\"reference#@!\"},\"image\":{\"docker-manifest-digest\":\"digest!@#\"},\"type\":\"atomic container signature\"},\"optional\":{\"creator\":\"CREATOR\",\"timestamp\":0}}"),
		marshaled)

	// We can't test MarshalJSON directly because the timestamp will keep changing, so just test that
	// it doesn't fail. And call it through the JSON package for a good measure.
	_, err = json.Marshal(s)
	assert.NoError(t, err)
}

// Return the result of modifying validJSON with fn and unmarshaling it into *sig
func tryUnmarshalModifiedSignature(t *testing.T, sig *privateSignature, validJSON []byte, modifyFn func(mSI)) error {
	var tmp mSI
	err := json.Unmarshal(validJSON, &tmp)
	require.NoError(t, err)

	modifyFn(tmp)

	testJSON, err := json.Marshal(tmp)
	require.NoError(t, err)

	*sig = privateSignature{}
	return json.Unmarshal(testJSON, sig)
}

func TestUnmarshalJSON(t *testing.T) {
	var s privateSignature
	// Invalid input. Note that json.Unmarshal is guaranteed to validate input before calling our
	// UnmarshalJSON implementation; so test that first, then test our error handling for completeness.
	err := json.Unmarshal([]byte("&"), &s)
	assert.Error(t, err)
	err = s.UnmarshalJSON([]byte("&"))
	assert.Error(t, err)

	// Not an object
	err = json.Unmarshal([]byte("1"), &s)
	assert.Error(t, err)

	// Start with a valid JSON.
	validSig := privateSignature{
		Signature{
			DockerManifestDigest: "digest!@#",
			DockerReference:      "reference#@!",
		},
	}
	validJSON, err := validSig.MarshalJSON()
	require.NoError(t, err)

	// Success
	s = privateSignature{}
	err = json.Unmarshal(validJSON, &s)
	require.NoError(t, err)
	assert.Equal(t, validSig, s)

	// Various ways to corrupt the JSON
	breakFns := []func(mSI){
		// A top-level field is missing
		func(v mSI) { delete(v, "critical") },
		func(v mSI) { delete(v, "optional") },
		// Extra top-level sub-object
		func(v mSI) { v["unexpected"] = 1 },
		// "critical" not an object
		func(v mSI) { v["critical"] = 1 },
		// "optional" not an object
		func(v mSI) { v["optional"] = 1 },
		// A field of "critical" is missing
		func(v mSI) { delete(x(v, "critical"), "type") },
		func(v mSI) { delete(x(v, "critical"), "image") },
		func(v mSI) { delete(x(v, "critical"), "identity") },
		// Extra field of "critical"
		func(v mSI) { x(v, "critical")["unexpected"] = 1 },
		// Invalid "type"
		func(v mSI) { x(v, "critical")["type"] = 1 },
		func(v mSI) { x(v, "critical")["type"] = "unexpected" },
		// Invalid "image" object
		func(v mSI) { x(v, "critical")["image"] = 1 },
		func(v mSI) { delete(x(v, "critical", "image"), "docker-manifest-digest") },
		func(v mSI) { x(v, "critical", "image")["unexpected"] = 1 },
		// Invalid "docker-manifest-digest"
		func(v mSI) { x(v, "critical", "image")["docker-manifest-digest"] = 1 },
		// Invalid "identity" object
		func(v mSI) { x(v, "critical")["identity"] = 1 },
		func(v mSI) { delete(x(v, "critical", "identity"), "docker-reference") },
		func(v mSI) { x(v, "critical", "identity")["unexpected"] = 1 },
		// Invalid "docker-reference"
		func(v mSI) { x(v, "critical", "identity")["docker-reference"] = 1 },
	}
	for _, fn := range breakFns {
		err = tryUnmarshalModifiedSignature(t, &s, validJSON, fn)
		assert.Error(t, err)
	}

	// Modifications to "optional" are allowed and ignored
	allowedModificationFns := []func(mSI){
		// Add an optional field
		func(v mSI) { x(v, "optional")["unexpected"] = 1 },
		// Delete an optional field
		func(v mSI) { delete(x(v, "optional"), "creator") },
	}
	for _, fn := range allowedModificationFns {
		err = tryUnmarshalModifiedSignature(t, &s, validJSON, fn)
		require.NoError(t, err)
		assert.Equal(t, validSig, s)
	}
}

func TestSign(t *testing.T) {
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)

	sig := privateSignature{
		Signature{
			DockerManifestDigest: "digest!@#",
			DockerReference:      "reference#@!",
		},
	}

	// Successful signing
	signature, err := sig.sign(mech, TestKeyFingerprint)
	require.NoError(t, err)

	verified, err := verifyAndExtractSignature(mech, signature, TestKeyFingerprint, sig.DockerReference)
	require.NoError(t, err)

	assert.Equal(t, sig.Signature, *verified)

	// Error creating blob to sign
	_, err = privateSignature{}.sign(mech, TestKeyFingerprint)
	assert.Error(t, err)

	// Error signing
	_, err = sig.sign(mech, "this fingerprint doesn't exist")
	assert.Error(t, err)
}

func TestVerifyAndExtractSignature(t *testing.T) {
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)

	signature, err := ioutil.ReadFile("./fixtures/image.signature")
	require.NoError(t, err)

	// Successful verification
	sig, err := verifyAndExtractSignature(mech, signature, TestKeyFingerprint, TestImageSignatureReference)
	require.NoError(t, err)
	assert.Equal(t, TestImageSignatureReference, sig.DockerReference)
	assert.Equal(t, TestImageManifestDigest, sig.DockerManifestDigest)

	// For extra paranoia, test that we return a nil signature object on error.

	// Completely invalid signature.
	sig, err = verifyAndExtractSignature(mech, []byte{}, TestKeyFingerprint, TestImageSignatureReference)
	assert.Error(t, err)
	assert.Nil(t, sig)

	sig, err = verifyAndExtractSignature(mech, []byte("invalid signature"), TestKeyFingerprint, TestImageSignatureReference)
	assert.Error(t, err)
	assert.Nil(t, sig)

	// Valid signature of non-JSON
	invalidBlobSignature, err := ioutil.ReadFile("./fixtures/invalid-blob.signature")
	require.NoError(t, err)
	sig, err = verifyAndExtractSignature(mech, invalidBlobSignature, TestKeyFingerprint, TestImageSignatureReference)
	assert.Error(t, err)
	assert.Nil(t, sig)

	// Valid signature with a wrong key
	sig, err = verifyAndExtractSignature(mech, signature, "unexpected fingerprint", TestImageSignatureReference)
	assert.Error(t, err)
	assert.Nil(t, sig)

	// Valid signature with a wrong image reference
	sig, err = verifyAndExtractSignature(mech, signature, TestKeyFingerprint, "unexpected docker reference")
	assert.Error(t, err)
	assert.Nil(t, sig)
}
