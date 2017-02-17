package signature

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/containers/image/directory"
	"github.com/containers/image/docker"
	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// policyFixtureContents is a data structure equal to the contents of "fixtures/policy.json"
var policyFixtureContents = &Policy{
	Default: PolicyRequirements{NewPRReject()},
	Transports: map[string]PolicyTransportScopes{
		"dir": {
			"": PolicyRequirements{NewPRInsecureAcceptAnything()},
		},
		"docker": {
			"example.com/playground": {
				NewPRInsecureAcceptAnything(),
			},
			"example.com/production": {
				xNewPRSignedByKeyPath(SBKeyTypeGPGKeys,
					"/keys/employee-gpg-keyring",
					NewPRMMatchRepoDigestOrExact()),
			},
			"example.com/hardened": {
				xNewPRSignedByKeyPath(SBKeyTypeGPGKeys,
					"/keys/employee-gpg-keyring",
					NewPRMMatchRepository()),
				xNewPRSignedByKeyPath(SBKeyTypeSignedByGPGKeys,
					"/keys/public-key-signing-gpg-keyring",
					NewPRMMatchExact()),
				xNewPRSignedBaseLayer(xNewPRMExactRepository("registry.access.redhat.com/rhel7/rhel")),
			},
			"example.com/hardened-x509": {
				xNewPRSignedByKeyPath(SBKeyTypeX509Certificates,
					"/keys/employee-cert-file",
					NewPRMMatchRepository()),
				xNewPRSignedByKeyPath(SBKeyTypeSignedByX509CAs,
					"/keys/public-key-signing-ca-file",
					NewPRMMatchRepoDigestOrExact()),
			},
			"registry.access.redhat.com": {
				xNewPRSignedByKeyPath(SBKeyTypeSignedByGPGKeys,
					"/keys/RH-key-signing-key-gpg-keyring",
					NewPRMMatchRepoDigestOrExact()),
			},
			"bogus/key-data-example": {
				xNewPRSignedByKeyData(SBKeyTypeSignedByGPGKeys,
					[]byte("nonsense"),
					NewPRMMatchRepoDigestOrExact()),
			},
			"bogus/signed-identity-example": {
				xNewPRSignedBaseLayer(xNewPRMExactReference("registry.access.redhat.com/rhel7/rhel:latest")),
			},
		},
	},
}

func TestDefaultPolicy(t *testing.T) {
	// We can't test the actual systemDefaultPolicyPath, so override.
	// TestDefaultPolicyPath below tests that we handle the overrides and defaults
	// correctly.

	// Success
	policy, err := DefaultPolicy(&types.SystemContext{SignaturePolicyPath: "./fixtures/policy.json"})
	require.NoError(t, err)
	assert.Equal(t, policyFixtureContents, policy)

	for _, path := range []string{
		"/this/doesnt/exist", // Error reading file
		"/dev/null",          // A failure case; most are tested in the individual method unit tests.
	} {
		policy, err := DefaultPolicy(&types.SystemContext{SignaturePolicyPath: path})
		assert.Error(t, err)
		assert.Nil(t, policy)
	}
}

func TestDefaultPolicyPath(t *testing.T) {

	const nondefaultPath = "/this/is/not/the/default/path.json"
	const variableReference = "$HOME"
	const rootPrefix = "/root/prefix"

	for _, c := range []struct {
		ctx      *types.SystemContext
		expected string
	}{
		// The common case
		{nil, systemDefaultPolicyPath},
		// There is a context, but it does not override the path.
		{&types.SystemContext{}, systemDefaultPolicyPath},
		// Path overridden
		{&types.SystemContext{SignaturePolicyPath: nondefaultPath}, nondefaultPath},
		// Root overridden
		{
			&types.SystemContext{RootForImplicitAbsolutePaths: rootPrefix},
			filepath.Join(rootPrefix, systemDefaultPolicyPath),
		},
		// Root and path overrides present simultaneously,
		{
			&types.SystemContext{
				RootForImplicitAbsolutePaths: rootPrefix,
				SignaturePolicyPath:          nondefaultPath,
			},
			nondefaultPath,
		},
		// No environment expansion happens in the overridden paths
		{&types.SystemContext{SignaturePolicyPath: variableReference}, variableReference},
	} {
		path := defaultPolicyPath(c.ctx)
		assert.Equal(t, c.expected, path)
	}
}

func TestNewPolicyFromFile(t *testing.T) {
	// Success
	policy, err := NewPolicyFromFile("./fixtures/policy.json")
	require.NoError(t, err)
	assert.Equal(t, policyFixtureContents, policy)

	// Error reading file
	_, err = NewPolicyFromFile("/this/doesnt/exist")
	assert.Error(t, err)

	// A failure case; most are tested in the individual method unit tests.
	_, err = NewPolicyFromFile("/dev/null")
	require.Error(t, err)
	assert.IsType(t, InvalidPolicyFormatError(""), err)
}

func TestNewPolicyFromBytes(t *testing.T) {
	// Success
	bytes, err := ioutil.ReadFile("./fixtures/policy.json")
	require.NoError(t, err)
	policy, err := NewPolicyFromBytes(bytes)
	require.NoError(t, err)
	assert.Equal(t, policyFixtureContents, policy)

	// A failure case; most are tested in the individual method unit tests.
	_, err = NewPolicyFromBytes([]byte(""))
	require.Error(t, err)
	assert.IsType(t, InvalidPolicyFormatError(""), err)
}

// FIXME? There is quite a bit of duplication below. Factor some of it out?

// testInvalidJSONInput verifies that obviously invalid input is rejected for dest.
func testInvalidJSONInput(t *testing.T, dest json.Unmarshaler) {
	// Invalid input. Note that json.Unmarshal is guaranteed to validate input before calling our
	// UnmarshalJSON implementation; so test that first, then test our error handling for completeness.
	err := json.Unmarshal([]byte("&"), dest)
	assert.Error(t, err)
	err = dest.UnmarshalJSON([]byte("&"))
	assert.Error(t, err)

	// Not an object/array/string
	err = json.Unmarshal([]byte("1"), dest)
	assert.Error(t, err)
}

// addExtraJSONMember adds adds an additional member "$name": $extra,
// possibly with a duplicate name, to encoded.
// Errors, if any, are reported through t.
func addExtraJSONMember(t *testing.T, encoded []byte, name string, extra interface{}) []byte {
	extraJSON, err := json.Marshal(extra)
	require.NoError(t, err)

	require.True(t, bytes.HasSuffix(encoded, []byte("}")))
	preservedLen := len(encoded) - 1

	return bytes.Join([][]byte{encoded[:preservedLen], []byte(`,"`), []byte(name), []byte(`":`), extraJSON, []byte("}")}, nil)
}

func TestInvalidPolicyFormatError(t *testing.T) {
	// A stupid test just to keep code coverage
	s := "test"
	err := InvalidPolicyFormatError(s)
	assert.Equal(t, s, err.Error())
}

// Return the result of modifying validJSON with fn and unmarshaling it into *p
func tryUnmarshalModifiedPolicy(t *testing.T, p *Policy, validJSON []byte, modifyFn func(mSI)) error {
	var tmp mSI
	err := json.Unmarshal(validJSON, &tmp)
	require.NoError(t, err)

	modifyFn(tmp)

	testJSON, err := json.Marshal(tmp)
	require.NoError(t, err)

	*p = Policy{}
	return json.Unmarshal(testJSON, p)
}

// xNewPRSignedByKeyPath is like NewPRSignedByKeyPath, except it must not fail.
func xNewPRSignedByKeyPath(keyType sbKeyType, keyPath string, signedIdentity PolicyReferenceMatch) PolicyRequirement {
	pr, err := NewPRSignedByKeyPath(keyType, keyPath, signedIdentity)
	if err != nil {
		panic("xNewPRSignedByKeyPath failed")
	}
	return pr
}

// xNewPRSignedByKeyData is like NewPRSignedByKeyData, except it must not fail.
func xNewPRSignedByKeyData(keyType sbKeyType, keyData []byte, signedIdentity PolicyReferenceMatch) PolicyRequirement {
	pr, err := NewPRSignedByKeyData(keyType, keyData, signedIdentity)
	if err != nil {
		panic("xNewPRSignedByKeyData failed")
	}
	return pr
}

func TestPolicyUnmarshalJSON(t *testing.T) {
	var p Policy

	testInvalidJSONInput(t, &p)

	// Start with a valid JSON.
	validPolicy := Policy{
		Default: []PolicyRequirement{
			xNewPRSignedByKeyData(SBKeyTypeGPGKeys, []byte("abc"), NewPRMMatchRepoDigestOrExact()),
		},
		Transports: map[string]PolicyTransportScopes{
			"docker": {
				"docker.io/library/busybox": []PolicyRequirement{
					xNewPRSignedByKeyData(SBKeyTypeGPGKeys, []byte("def"), NewPRMMatchRepoDigestOrExact()),
				},
				"registry.access.redhat.com": []PolicyRequirement{
					xNewPRSignedByKeyData(SBKeyTypeSignedByGPGKeys, []byte("RH"), NewPRMMatchRepository()),
				},
			},
			"atomic": {
				"registry.access.redhat.com/rhel7": []PolicyRequirement{
					xNewPRSignedByKeyData(SBKeyTypeSignedByGPGKeys, []byte("RHatomic"), NewPRMMatchRepository()),
				},
			},
		},
	}
	validJSON, err := json.Marshal(validPolicy)
	require.NoError(t, err)

	// Success
	p = Policy{}
	err = json.Unmarshal(validJSON, &p)
	require.NoError(t, err)
	assert.Equal(t, validPolicy, p)

	// Various ways to corrupt the JSON
	breakFns := []func(mSI){
		// The "default" field is missing
		func(v mSI) { delete(v, "default") },
		// Extra top-level sub-object
		func(v mSI) { v["unexpected"] = 1 },
		// "default" not an array
		func(v mSI) { v["default"] = 1 },
		func(v mSI) { v["default"] = mSI{} },
		// "transports" not an object
		func(v mSI) { v["transports"] = 1 },
		func(v mSI) { v["transports"] = []string{} },
		// "default" is an invalid PolicyRequirements
		func(v mSI) { v["default"] = PolicyRequirements{} },
		// A key in "transports" is an invalid transport name
		func(v mSI) { x(v, "transports")["this is unknown"] = x(v, "transports")["docker"] },
		func(v mSI) { x(v, "transports")[""] = x(v, "transports")["docker"] },
	}
	for _, fn := range breakFns {
		err = tryUnmarshalModifiedPolicy(t, &p, validJSON, fn)
		assert.Error(t, err)
	}

	// Duplicated fields
	for _, field := range []string{"default", "transports"} {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		testJSON := addExtraJSONMember(t, validJSON, field, tmp[field])

		p = Policy{}
		err = json.Unmarshal(testJSON, &p)
		assert.Error(t, err)
	}

	// Various allowed modifications to the policy
	allowedModificationFns := []func(mSI){
		// Delete the map of specific policies
		func(v mSI) { delete(v, "specific") },
		// Use an empty map of transport-specific scopes
		func(v mSI) { v["transports"] = map[string]PolicyTransportScopes{} },
	}
	for _, fn := range allowedModificationFns {
		err = tryUnmarshalModifiedPolicy(t, &p, validJSON, fn)
		require.NoError(t, err)
	}
}

func TestPolicyTransportScopesUnmarshalJSON(t *testing.T) {
	var pts PolicyTransportScopes

	// Start with a valid JSON.
	validPTS := PolicyTransportScopes{
		"": []PolicyRequirement{
			xNewPRSignedByKeyData(SBKeyTypeGPGKeys, []byte("global"), NewPRMMatchRepoDigestOrExact()),
		},
	}
	validJSON, err := json.Marshal(validPTS)
	require.NoError(t, err)

	// Nothing can be unmarshaled directly into PolicyTransportScopes
	pts = PolicyTransportScopes{}
	err = json.Unmarshal(validJSON, &pts)
	assert.Error(t, err)
}

// Return the result of modifying validJSON with fn and unmarshaling it into *pts
// using transport.
func tryUnmarshalModifiedPTS(t *testing.T, pts *PolicyTransportScopes, transport types.ImageTransport,
	validJSON []byte, modifyFn func(mSI)) error {
	var tmp mSI
	err := json.Unmarshal(validJSON, &tmp)
	require.NoError(t, err)

	modifyFn(tmp)

	testJSON, err := json.Marshal(tmp)
	require.NoError(t, err)

	*pts = PolicyTransportScopes{}
	dest := policyTransportScopesWithTransport{
		transport: transport,
		dest:      pts,
	}
	return json.Unmarshal(testJSON, &dest)
}

func TestPolicyTransportScopesWithTransportUnmarshalJSON(t *testing.T) {
	var pts PolicyTransportScopes

	dest := policyTransportScopesWithTransport{
		transport: docker.Transport,
		dest:      &pts,
	}
	testInvalidJSONInput(t, &dest)

	// Start with a valid JSON.
	validPTS := PolicyTransportScopes{
		"docker.io/library/busybox": []PolicyRequirement{
			xNewPRSignedByKeyData(SBKeyTypeGPGKeys, []byte("def"), NewPRMMatchRepoDigestOrExact()),
		},
		"registry.access.redhat.com": []PolicyRequirement{
			xNewPRSignedByKeyData(SBKeyTypeSignedByGPGKeys, []byte("RH"), NewPRMMatchRepository()),
		},
		"": []PolicyRequirement{
			xNewPRSignedByKeyData(SBKeyTypeGPGKeys, []byte("global"), NewPRMMatchRepoDigestOrExact()),
		},
	}
	validJSON, err := json.Marshal(validPTS)
	require.NoError(t, err)

	// Success
	pts = PolicyTransportScopes{}
	dest = policyTransportScopesWithTransport{
		transport: docker.Transport,
		dest:      &pts,
	}
	err = json.Unmarshal(validJSON, &dest)
	require.NoError(t, err)
	assert.Equal(t, validPTS, pts)

	// Various ways to corrupt the JSON
	breakFns := []func(mSI){
		// A scope is not an array
		func(v mSI) { v["docker.io/library/busybox"] = 1 },
		func(v mSI) { v["docker.io/library/busybox"] = mSI{} },
		func(v mSI) { v[""] = 1 },
		func(v mSI) { v[""] = mSI{} },
		// A scope is an invalid PolicyRequirements
		func(v mSI) { v["docker.io/library/busybox"] = PolicyRequirements{} },
		func(v mSI) { v[""] = PolicyRequirements{} },
	}
	for _, fn := range breakFns {
		err = tryUnmarshalModifiedPTS(t, &pts, docker.Transport, validJSON, fn)
		assert.Error(t, err)
	}

	// Duplicated fields
	for _, field := range []string{"docker.io/library/busybox", ""} {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		testJSON := addExtraJSONMember(t, validJSON, field, tmp[field])

		pts = PolicyTransportScopes{}
		dest := policyTransportScopesWithTransport{
			transport: docker.Transport,
			dest:      &pts,
		}
		err = json.Unmarshal(testJSON, &dest)
		assert.Error(t, err)
	}

	// Scope rejected by transport the Docker scopes we use as valid are rejected by directory.Transport
	// as relative paths.
	err = tryUnmarshalModifiedPTS(t, &pts, directory.Transport, validJSON,
		func(v mSI) {})
	assert.Error(t, err)

	// Various allowed modifications to the policy
	allowedModificationFns := []func(mSI){
		// The "" scope is missing
		func(v mSI) { delete(v, "") },
		// The policy is completely empty
		func(v mSI) {
			for key := range v {
				delete(v, key)
			}
		},
	}
	for _, fn := range allowedModificationFns {
		err = tryUnmarshalModifiedPTS(t, &pts, docker.Transport, validJSON, fn)
		require.NoError(t, err)
	}
}

func TestPolicyRequirementsUnmarshalJSON(t *testing.T) {
	var reqs PolicyRequirements

	testInvalidJSONInput(t, &reqs)

	// Start with a valid JSON.
	validReqs := PolicyRequirements{
		xNewPRSignedByKeyData(SBKeyTypeGPGKeys, []byte("def"), NewPRMMatchRepoDigestOrExact()),
		xNewPRSignedByKeyData(SBKeyTypeSignedByGPGKeys, []byte("RH"), NewPRMMatchRepository()),
	}
	validJSON, err := json.Marshal(validReqs)
	require.NoError(t, err)

	// Success
	reqs = PolicyRequirements{}
	err = json.Unmarshal(validJSON, &reqs)
	require.NoError(t, err)
	assert.Equal(t, validReqs, reqs)

	for _, invalid := range [][]interface{}{
		// No requirements
		{},
		// A member is not an object
		{1},
		// A member has an invalid type
		{prSignedBy{prCommon: prCommon{Type: "this is invalid"}}},
		// A member has a valid type but invalid contents
		{prSignedBy{
			prCommon: prCommon{Type: prTypeSignedBy},
			KeyType:  "this is invalid",
		}},
	} {
		testJSON, err := json.Marshal(invalid)
		require.NoError(t, err)

		reqs = PolicyRequirements{}
		err = json.Unmarshal(testJSON, &reqs)
		assert.Error(t, err, string(testJSON))
	}
}

func TestNewPolicyRequirementFromJSON(t *testing.T) {
	// Sample success. Others tested in the individual PolicyRequirement.UnmarshalJSON implementations.
	validReq := NewPRInsecureAcceptAnything()
	validJSON, err := json.Marshal(validReq)
	require.NoError(t, err)
	req, err := newPolicyRequirementFromJSON(validJSON)
	require.NoError(t, err)
	assert.Equal(t, validReq, req)

	// Invalid
	for _, invalid := range []interface{}{
		// Not an object
		1,
		// Missing type
		prCommon{},
		// Invalid type
		prCommon{Type: "this is invalid"},
		// Valid type but invalid contents
		prSignedBy{
			prCommon: prCommon{Type: prTypeSignedBy},
			KeyType:  "this is invalid",
		},
	} {
		testJSON, err := json.Marshal(invalid)
		require.NoError(t, err)

		_, err = newPolicyRequirementFromJSON(testJSON)
		assert.Error(t, err, string(testJSON))
	}
}

func TestNewPRInsecureAcceptAnything(t *testing.T) {
	_pr := NewPRInsecureAcceptAnything()
	pr, ok := _pr.(*prInsecureAcceptAnything)
	require.True(t, ok)
	assert.Equal(t, &prInsecureAcceptAnything{prCommon{prTypeInsecureAcceptAnything}}, pr)
}

func TestPRInsecureAcceptAnythingUnmarshalJSON(t *testing.T) {
	var pr prInsecureAcceptAnything

	testInvalidJSONInput(t, &pr)

	// Start with a valid JSON.
	validPR := NewPRInsecureAcceptAnything()
	validJSON, err := json.Marshal(validPR)
	require.NoError(t, err)

	// Success
	pr = prInsecureAcceptAnything{}
	err = json.Unmarshal(validJSON, &pr)
	require.NoError(t, err)
	assert.Equal(t, validPR, &pr)

	// newPolicyRequirementFromJSON recognizes this type
	_pr, err := newPolicyRequirementFromJSON(validJSON)
	require.NoError(t, err)
	assert.Equal(t, validPR, _pr)

	for _, invalid := range []mSI{
		// Missing "type" field
		{},
		// Wrong "type" field
		{"type": 1},
		{"type": "this is invalid"},
		// Extra fields
		{
			"type":    string(prTypeInsecureAcceptAnything),
			"unknown": "foo",
		},
	} {
		testJSON, err := json.Marshal(invalid)
		require.NoError(t, err)

		pr = prInsecureAcceptAnything{}
		err = json.Unmarshal(testJSON, &pr)
		assert.Error(t, err, string(testJSON))
	}

	// Duplicated fields
	for _, field := range []string{"type"} {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		testJSON := addExtraJSONMember(t, validJSON, field, tmp[field])

		pr = prInsecureAcceptAnything{}
		err = json.Unmarshal(testJSON, &pr)
		assert.Error(t, err)
	}
}

func TestNewPRReject(t *testing.T) {
	_pr := NewPRReject()
	pr, ok := _pr.(*prReject)
	require.True(t, ok)
	assert.Equal(t, &prReject{prCommon{prTypeReject}}, pr)
}

func TestPRRejectUnmarshalJSON(t *testing.T) {
	var pr prReject

	testInvalidJSONInput(t, &pr)

	// Start with a valid JSON.
	validPR := NewPRReject()
	validJSON, err := json.Marshal(validPR)
	require.NoError(t, err)

	// Success
	pr = prReject{}
	err = json.Unmarshal(validJSON, &pr)
	require.NoError(t, err)
	assert.Equal(t, validPR, &pr)

	// newPolicyRequirementFromJSON recognizes this type
	_pr, err := newPolicyRequirementFromJSON(validJSON)
	require.NoError(t, err)
	assert.Equal(t, validPR, _pr)

	for _, invalid := range []mSI{
		// Missing "type" field
		{},
		// Wrong "type" field
		{"type": 1},
		{"type": "this is invalid"},
		// Extra fields
		{
			"type":    string(prTypeReject),
			"unknown": "foo",
		},
	} {
		testJSON, err := json.Marshal(invalid)
		require.NoError(t, err)

		pr = prReject{}
		err = json.Unmarshal(testJSON, &pr)
		assert.Error(t, err, string(testJSON))
	}

	// Duplicated fields
	for _, field := range []string{"type"} {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		testJSON := addExtraJSONMember(t, validJSON, field, tmp[field])

		pr = prReject{}
		err = json.Unmarshal(testJSON, &pr)
		assert.Error(t, err)
	}
}

func TestNewPRSignedBy(t *testing.T) {
	const testPath = "/foo/bar"
	testData := []byte("abc")
	testIdentity := NewPRMMatchRepoDigestOrExact()

	// Success
	pr, err := newPRSignedBy(SBKeyTypeGPGKeys, testPath, nil, testIdentity)
	require.NoError(t, err)
	assert.Equal(t, &prSignedBy{
		prCommon:       prCommon{prTypeSignedBy},
		KeyType:        SBKeyTypeGPGKeys,
		KeyPath:        testPath,
		KeyData:        nil,
		SignedIdentity: testIdentity,
	}, pr)
	pr, err = newPRSignedBy(SBKeyTypeGPGKeys, "", testData, testIdentity)
	require.NoError(t, err)
	assert.Equal(t, &prSignedBy{
		prCommon:       prCommon{prTypeSignedBy},
		KeyType:        SBKeyTypeGPGKeys,
		KeyPath:        "",
		KeyData:        testData,
		SignedIdentity: testIdentity,
	}, pr)

	// Invalid keyType
	pr, err = newPRSignedBy(sbKeyType(""), testPath, nil, testIdentity)
	assert.Error(t, err)
	pr, err = newPRSignedBy(sbKeyType("this is invalid"), testPath, nil, testIdentity)
	assert.Error(t, err)

	// Both keyPath and keyData specified
	pr, err = newPRSignedBy(SBKeyTypeGPGKeys, testPath, testData, testIdentity)
	assert.Error(t, err)

	// Invalid signedIdentity
	pr, err = newPRSignedBy(SBKeyTypeGPGKeys, testPath, nil, nil)
	assert.Error(t, err)
}

func TestNewPRSignedByKeyPath(t *testing.T) {
	const testPath = "/foo/bar"
	_pr, err := NewPRSignedByKeyPath(SBKeyTypeGPGKeys, testPath, NewPRMMatchRepoDigestOrExact())
	require.NoError(t, err)
	pr, ok := _pr.(*prSignedBy)
	require.True(t, ok)
	assert.Equal(t, testPath, pr.KeyPath)
	// Failure cases tested in TestNewPRSignedBy.
}

func TestNewPRSignedByKeyData(t *testing.T) {
	testData := []byte("abc")
	_pr, err := NewPRSignedByKeyData(SBKeyTypeGPGKeys, testData, NewPRMMatchRepoDigestOrExact())
	require.NoError(t, err)
	pr, ok := _pr.(*prSignedBy)
	require.True(t, ok)
	assert.Equal(t, testData, pr.KeyData)
	// Failure cases tested in TestNewPRSignedBy.
}

// Return the result of modifying vaoidJSON with fn and unmarshalingit into *pr
func tryUnmarshalModifiedSignedBy(t *testing.T, pr *prSignedBy, validJSON []byte, modifyFn func(mSI)) error {
	var tmp mSI
	err := json.Unmarshal(validJSON, &tmp)
	require.NoError(t, err)

	modifyFn(tmp)

	testJSON, err := json.Marshal(tmp)
	require.NoError(t, err)

	*pr = prSignedBy{}
	return json.Unmarshal(testJSON, &pr)
}

func TestPRSignedByUnmarshalJSON(t *testing.T) {
	var pr prSignedBy

	testInvalidJSONInput(t, &pr)

	// Start with a valid JSON.
	validPR, err := NewPRSignedByKeyData(SBKeyTypeGPGKeys, []byte("abc"), NewPRMMatchRepoDigestOrExact())
	require.NoError(t, err)
	validJSON, err := json.Marshal(validPR)
	require.NoError(t, err)

	// Success with KeyData
	pr = prSignedBy{}
	err = json.Unmarshal(validJSON, &pr)
	require.NoError(t, err)
	assert.Equal(t, validPR, &pr)

	// Success with KeyPath
	kpPR, err := NewPRSignedByKeyPath(SBKeyTypeGPGKeys, "/foo/bar", NewPRMMatchRepoDigestOrExact())
	require.NoError(t, err)
	testJSON, err := json.Marshal(kpPR)
	require.NoError(t, err)
	pr = prSignedBy{}
	err = json.Unmarshal(testJSON, &pr)
	require.NoError(t, err)
	assert.Equal(t, kpPR, &pr)

	// newPolicyRequirementFromJSON recognizes this type
	_pr, err := newPolicyRequirementFromJSON(validJSON)
	require.NoError(t, err)
	assert.Equal(t, validPR, _pr)

	// Various ways to corrupt the JSON
	breakFns := []func(mSI){
		// The "type" field is missing
		func(v mSI) { delete(v, "type") },
		// Wrong "type" field
		func(v mSI) { v["type"] = 1 },
		func(v mSI) { v["type"] = "this is invalid" },
		// Extra top-level sub-object
		func(v mSI) { v["unexpected"] = 1 },
		// The "keyType" field is missing
		func(v mSI) { delete(v, "keyType") },
		// Invalid "keyType" field
		func(v mSI) { v["keyType"] = "this is invalid" },
		// Both "keyPath" and "keyData" is missing
		func(v mSI) { delete(v, "keyData") },
		// Both "keyPath" and "keyData" is present
		func(v mSI) { v["keyPath"] = "/foo/bar" },
		// Invalid "keyPath" field
		func(v mSI) { delete(v, "keyData"); v["keyPath"] = 1 },
		func(v mSI) { v["type"] = "this is invalid" },
		// Invalid "keyData" field
		func(v mSI) { v["keyData"] = 1 },
		func(v mSI) { v["keyData"] = "this is invalid base64" },
		// Invalid "signedIdentity" field
		func(v mSI) { v["signedIdentity"] = "this is invalid" },
		// "signedIdentity" an explicit nil
		func(v mSI) { v["signedIdentity"] = nil },
	}
	for _, fn := range breakFns {
		err = tryUnmarshalModifiedSignedBy(t, &pr, validJSON, fn)
		assert.Error(t, err, string(testJSON))
	}

	// Duplicated fields
	for _, field := range []string{"type", "keyType", "keyData", "signedIdentity"} {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		testJSON := addExtraJSONMember(t, validJSON, field, tmp[field])

		pr = prSignedBy{}
		err = json.Unmarshal(testJSON, &pr)
		assert.Error(t, err)
	}
	// Handle "keyPath", which is not in validJSON, specially
	pathPR, err := NewPRSignedByKeyPath(SBKeyTypeGPGKeys, "/foo/bar", NewPRMMatchRepoDigestOrExact())
	require.NoError(t, err)
	testJSON, err = json.Marshal(pathPR)
	require.NoError(t, err)
	testJSON = addExtraJSONMember(t, testJSON, "keyPath", pr.KeyPath)
	pr = prSignedBy{}
	err = json.Unmarshal(testJSON, &pr)
	assert.Error(t, err)

	// Various allowed modifications to the requirement
	allowedModificationFns := []func(mSI){
		// Delete the signedIdentity field
		func(v mSI) { delete(v, "signedIdentity") },
	}
	for _, fn := range allowedModificationFns {
		err = tryUnmarshalModifiedSignedBy(t, &pr, validJSON, fn)
		require.NoError(t, err)
	}

	// Various ways to set signedIdentity to the default value
	signedIdentityDefaultFns := []func(mSI){
		// Set signedIdentity to the default explicitly
		func(v mSI) { v["signedIdentity"] = NewPRMMatchRepoDigestOrExact() },
		// Delete the signedIdentity field
		func(v mSI) { delete(v, "signedIdentity") },
	}
	for _, fn := range signedIdentityDefaultFns {
		err = tryUnmarshalModifiedSignedBy(t, &pr, validJSON, fn)
		require.NoError(t, err)
		assert.Equal(t, NewPRMMatchRepoDigestOrExact(), pr.SignedIdentity)
	}
}

func TestSBKeyTypeIsValid(t *testing.T) {
	// Valid values
	for _, s := range []sbKeyType{
		SBKeyTypeGPGKeys,
		SBKeyTypeSignedByGPGKeys,
		SBKeyTypeX509Certificates,
		SBKeyTypeSignedByX509CAs,
	} {
		assert.True(t, s.IsValid())
	}

	// Invalid values
	for _, s := range []string{"", "this is invalid"} {
		assert.False(t, sbKeyType(s).IsValid())
	}
}

func TestSBKeyTypeUnmarshalJSON(t *testing.T) {
	var kt sbKeyType

	testInvalidJSONInput(t, &kt)

	// Valid values.
	for _, v := range []sbKeyType{
		SBKeyTypeGPGKeys,
		SBKeyTypeSignedByGPGKeys,
		SBKeyTypeX509Certificates,
		SBKeyTypeSignedByX509CAs,
	} {
		kt = sbKeyType("")
		err := json.Unmarshal([]byte(`"`+string(v)+`"`), &kt)
		assert.NoError(t, err)
	}

	// Invalid values
	kt = sbKeyType("")
	err := json.Unmarshal([]byte(`""`), &kt)
	assert.Error(t, err)

	kt = sbKeyType("")
	err = json.Unmarshal([]byte(`"this is invalid"`), &kt)
	assert.Error(t, err)
}

// NewPRSignedBaseLayer is like NewPRSignedBaseLayer, except it must not fail.
func xNewPRSignedBaseLayer(baseLayerIdentity PolicyReferenceMatch) PolicyRequirement {
	pr, err := NewPRSignedBaseLayer(baseLayerIdentity)
	if err != nil {
		panic("xNewPRSignedBaseLayer failed")
	}
	return pr
}

func TestNewPRSignedBaseLayer(t *testing.T) {
	testBLI := NewPRMMatchExact()

	// Success
	_pr, err := NewPRSignedBaseLayer(testBLI)
	require.NoError(t, err)
	pr, ok := _pr.(*prSignedBaseLayer)
	require.True(t, ok)
	assert.Equal(t, &prSignedBaseLayer{
		prCommon:          prCommon{prTypeSignedBaseLayer},
		BaseLayerIdentity: testBLI,
	}, pr)

	// Invalid baseLayerIdentity
	_, err = NewPRSignedBaseLayer(nil)
	assert.Error(t, err)
}

func TestPRSignedBaseLayerUnmarshalJSON(t *testing.T) {
	var pr prSignedBaseLayer

	testInvalidJSONInput(t, &pr)

	// Start with a valid JSON.
	baseIdentity, err := NewPRMExactReference("registry.access.redhat.com/rhel7/rhel:7.2.3")
	require.NoError(t, err)
	validPR, err := NewPRSignedBaseLayer(baseIdentity)
	require.NoError(t, err)
	validJSON, err := json.Marshal(validPR)
	require.NoError(t, err)

	// Success
	pr = prSignedBaseLayer{}
	err = json.Unmarshal(validJSON, &pr)
	require.NoError(t, err)
	assert.Equal(t, validPR, &pr)

	// newPolicyRequirementFromJSON recognizes this type
	_pr, err := newPolicyRequirementFromJSON(validJSON)
	require.NoError(t, err)
	assert.Equal(t, validPR, _pr)

	// Various ways to corrupt the JSON
	breakFns := []func(mSI){
		// The "type" field is missing
		func(v mSI) { delete(v, "type") },
		// Wrong "type" field
		func(v mSI) { v["type"] = 1 },
		func(v mSI) { v["type"] = "this is invalid" },
		// Extra top-level sub-object
		func(v mSI) { v["unexpected"] = 1 },
		// The "baseLayerIdentity" field is missing
		func(v mSI) { delete(v, "baseLayerIdentity") },
		// Invalid "baseLayerIdentity" field
		func(v mSI) { v["baseLayerIdentity"] = "this is invalid" },
		// Invalid "baseLayerIdentity" an explicit nil
		func(v mSI) { v["baseLayerIdentity"] = nil },
	}
	for _, fn := range breakFns {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		fn(tmp)

		testJSON, err := json.Marshal(tmp)
		require.NoError(t, err)

		pr = prSignedBaseLayer{}
		err = json.Unmarshal(testJSON, &pr)
		assert.Error(t, err)
	}

	// Duplicated fields
	for _, field := range []string{"type", "baseLayerIdentity"} {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		testJSON := addExtraJSONMember(t, validJSON, field, tmp[field])

		pr = prSignedBaseLayer{}
		err = json.Unmarshal(testJSON, &pr)
		assert.Error(t, err)
	}
}

func TestNewPolicyReferenceMatchFromJSON(t *testing.T) {
	// Sample success. Others tested in the individual PolicyReferenceMatch.UnmarshalJSON implementations.
	validPRM := NewPRMMatchRepoDigestOrExact()
	validJSON, err := json.Marshal(validPRM)
	require.NoError(t, err)
	prm, err := newPolicyReferenceMatchFromJSON(validJSON)
	require.NoError(t, err)
	assert.Equal(t, validPRM, prm)

	// Invalid
	for _, invalid := range []interface{}{
		// Not an object
		1,
		// Missing type
		prmCommon{},
		// Invalid type
		prmCommon{Type: "this is invalid"},
		// Valid type but invalid contents
		prmExactReference{
			prmCommon:       prmCommon{Type: prmTypeExactReference},
			DockerReference: "",
		},
	} {
		testJSON, err := json.Marshal(invalid)
		require.NoError(t, err)

		_, err = newPolicyReferenceMatchFromJSON(testJSON)
		assert.Error(t, err, string(testJSON))
	}
}

func TestNewPRMMatchExact(t *testing.T) {
	_prm := NewPRMMatchExact()
	prm, ok := _prm.(*prmMatchExact)
	require.True(t, ok)
	assert.Equal(t, &prmMatchExact{prmCommon{prmTypeMatchExact}}, prm)
}

func TestPRMMatchExactUnmarshalJSON(t *testing.T) {
	var prm prmMatchExact

	testInvalidJSONInput(t, &prm)

	// Start with a valid JSON.
	validPR := NewPRMMatchExact()
	validJSON, err := json.Marshal(validPR)
	require.NoError(t, err)

	// Success
	prm = prmMatchExact{}
	err = json.Unmarshal(validJSON, &prm)
	require.NoError(t, err)
	assert.Equal(t, validPR, &prm)

	// newPolicyReferenceMatchFromJSON recognizes this type
	_pr, err := newPolicyReferenceMatchFromJSON(validJSON)
	require.NoError(t, err)
	assert.Equal(t, validPR, _pr)

	for _, invalid := range []mSI{
		// Missing "type" field
		{},
		// Wrong "type" field
		{"type": 1},
		{"type": "this is invalid"},
		// Extra fields
		{
			"type":    string(prmTypeMatchExact),
			"unknown": "foo",
		},
	} {
		testJSON, err := json.Marshal(invalid)
		require.NoError(t, err)

		prm = prmMatchExact{}
		err = json.Unmarshal(testJSON, &prm)
		assert.Error(t, err, string(testJSON))
	}

	// Duplicated fields
	for _, field := range []string{"type"} {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		testJSON := addExtraJSONMember(t, validJSON, field, tmp[field])

		prm = prmMatchExact{}
		err = json.Unmarshal(testJSON, &prm)
		assert.Error(t, err)
	}
}

func TestNewPRMMatchRepoDigestOrExact(t *testing.T) {
	_prm := NewPRMMatchRepoDigestOrExact()
	prm, ok := _prm.(*prmMatchRepoDigestOrExact)
	require.True(t, ok)
	assert.Equal(t, &prmMatchRepoDigestOrExact{prmCommon{prmTypeMatchRepoDigestOrExact}}, prm)
}

func TestPRMMatchRepoDigestOrExactUnmarshalJSON(t *testing.T) {
	var prm prmMatchRepoDigestOrExact

	testInvalidJSONInput(t, &prm)

	// Start with a valid JSON.
	validPR := NewPRMMatchRepoDigestOrExact()
	validJSON, err := json.Marshal(validPR)
	require.NoError(t, err)

	// Success
	prm = prmMatchRepoDigestOrExact{}
	err = json.Unmarshal(validJSON, &prm)
	require.NoError(t, err)
	assert.Equal(t, validPR, &prm)

	// newPolicyReferenceMatchFromJSON recognizes this type
	_pr, err := newPolicyReferenceMatchFromJSON(validJSON)
	require.NoError(t, err)
	assert.Equal(t, validPR, _pr)

	for _, invalid := range []mSI{
		// Missing "type" field
		{},
		// Wrong "type" field
		{"type": 1},
		{"type": "this is invalid"},
		// Extra fields
		{
			"type":    string(prmTypeMatchRepoDigestOrExact),
			"unknown": "foo",
		},
	} {
		testJSON, err := json.Marshal(invalid)
		require.NoError(t, err)

		prm = prmMatchRepoDigestOrExact{}
		err = json.Unmarshal(testJSON, &prm)
		assert.Error(t, err, string(testJSON))
	}

	// Duplicated fields
	for _, field := range []string{"type"} {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		testJSON := addExtraJSONMember(t, validJSON, field, tmp[field])

		prm = prmMatchRepoDigestOrExact{}
		err = json.Unmarshal(testJSON, &prm)
		assert.Error(t, err)
	}
}

func TestNewPRMMatchRepository(t *testing.T) {
	_prm := NewPRMMatchRepository()
	prm, ok := _prm.(*prmMatchRepository)
	require.True(t, ok)
	assert.Equal(t, &prmMatchRepository{prmCommon{prmTypeMatchRepository}}, prm)
}

func TestPRMMatchRepositoryUnmarshalJSON(t *testing.T) {
	var prm prmMatchRepository

	testInvalidJSONInput(t, &prm)

	// Start with a valid JSON.
	validPR := NewPRMMatchRepository()
	validJSON, err := json.Marshal(validPR)
	require.NoError(t, err)

	// Success
	prm = prmMatchRepository{}
	err = json.Unmarshal(validJSON, &prm)
	require.NoError(t, err)
	assert.Equal(t, validPR, &prm)

	// newPolicyReferenceMatchFromJSON recognizes this type
	_pr, err := newPolicyReferenceMatchFromJSON(validJSON)
	require.NoError(t, err)
	assert.Equal(t, validPR, _pr)

	for _, invalid := range []mSI{
		// Missing "type" field
		{},
		// Wrong "type" field
		{"type": 1},
		{"type": "this is invalid"},
		// Extra fields
		{
			"type":    string(prmTypeMatchRepository),
			"unknown": "foo",
		},
	} {
		testJSON, err := json.Marshal(invalid)
		require.NoError(t, err)

		prm = prmMatchRepository{}
		err = json.Unmarshal(testJSON, &prm)
		assert.Error(t, err, string(testJSON))
	}

	// Duplicated fields
	for _, field := range []string{"type"} {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		testJSON := addExtraJSONMember(t, validJSON, field, tmp[field])

		prm = prmMatchRepository{}
		err = json.Unmarshal(testJSON, &prm)
		assert.Error(t, err)
	}
}

// xNewPRMExactReference is like NewPRMExactReference, except it must not fail.
func xNewPRMExactReference(dockerReference string) PolicyReferenceMatch {
	pr, err := NewPRMExactReference(dockerReference)
	if err != nil {
		panic("xNewPRMExactReference failed")
	}
	return pr
}

func TestNewPRMExactReference(t *testing.T) {
	const testDR = "library/busybox:latest"

	// Success
	_prm, err := NewPRMExactReference(testDR)
	require.NoError(t, err)
	prm, ok := _prm.(*prmExactReference)
	require.True(t, ok)
	assert.Equal(t, &prmExactReference{
		prmCommon:       prmCommon{prmTypeExactReference},
		DockerReference: testDR,
	}, prm)

	// Invalid dockerReference
	_, err = NewPRMExactReference("")
	assert.Error(t, err)
	// Uppercase is invalid in Docker reference components.
	_, err = NewPRMExactReference("INVALIDUPPERCASE:latest")
	assert.Error(t, err)
	// Missing tag
	_, err = NewPRMExactReference("library/busybox")
	assert.Error(t, err)
}

func TestPRMExactReferenceUnmarshalJSON(t *testing.T) {
	var prm prmExactReference

	testInvalidJSONInput(t, &prm)

	// Start with a valid JSON.
	validPRM, err := NewPRMExactReference("library/buxybox:latest")
	require.NoError(t, err)
	validJSON, err := json.Marshal(validPRM)
	require.NoError(t, err)

	// Success
	prm = prmExactReference{}
	err = json.Unmarshal(validJSON, &prm)
	require.NoError(t, err)
	assert.Equal(t, validPRM, &prm)

	// newPolicyReferenceMatchFromJSON recognizes this type
	_prm, err := newPolicyReferenceMatchFromJSON(validJSON)
	require.NoError(t, err)
	assert.Equal(t, validPRM, _prm)

	// Various ways to corrupt the JSON
	breakFns := []func(mSI){
		// The "type" field is missing
		func(v mSI) { delete(v, "type") },
		// Wrong "type" field
		func(v mSI) { v["type"] = 1 },
		func(v mSI) { v["type"] = "this is invalid" },
		// Extra top-level sub-object
		func(v mSI) { v["unexpected"] = 1 },
		// The "dockerReference" field is missing
		func(v mSI) { delete(v, "dockerReference") },
		// Invalid "dockerReference" field
		func(v mSI) { v["dockerReference"] = 1 },
	}
	for _, fn := range breakFns {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		fn(tmp)

		testJSON, err := json.Marshal(tmp)
		require.NoError(t, err)

		prm = prmExactReference{}
		err = json.Unmarshal(testJSON, &prm)
		assert.Error(t, err)
	}

	// Duplicated fields
	for _, field := range []string{"type", "baseLayerIdentity"} {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		testJSON := addExtraJSONMember(t, validJSON, field, tmp[field])

		prm = prmExactReference{}
		err = json.Unmarshal(testJSON, &prm)
		assert.Error(t, err)
	}
}

// xNewPRMExactRepository is like NewPRMExactRepository, except it must not fail.
func xNewPRMExactRepository(dockerRepository string) PolicyReferenceMatch {
	pr, err := NewPRMExactRepository(dockerRepository)
	if err != nil {
		panic("xNewPRMExactRepository failed")
	}
	return pr
}

func TestNewPRMExactRepository(t *testing.T) {
	const testDR = "library/busybox:latest"

	// Success
	_prm, err := NewPRMExactRepository(testDR)
	require.NoError(t, err)
	prm, ok := _prm.(*prmExactRepository)
	require.True(t, ok)
	assert.Equal(t, &prmExactRepository{
		prmCommon:        prmCommon{prmTypeExactRepository},
		DockerRepository: testDR,
	}, prm)

	// Invalid dockerRepository
	_, err = NewPRMExactRepository("")
	assert.Error(t, err)
	// Uppercase is invalid in Docker reference components.
	_, err = NewPRMExactRepository("INVALIDUPPERCASE")
	assert.Error(t, err)
}

func TestPRMExactRepositoryUnmarshalJSON(t *testing.T) {
	var prm prmExactRepository

	testInvalidJSONInput(t, &prm)

	// Start with a valid JSON.
	validPRM, err := NewPRMExactRepository("library/buxybox:latest")
	require.NoError(t, err)
	validJSON, err := json.Marshal(validPRM)
	require.NoError(t, err)

	// Success
	prm = prmExactRepository{}
	err = json.Unmarshal(validJSON, &prm)
	require.NoError(t, err)
	assert.Equal(t, validPRM, &prm)

	// newPolicyReferenceMatchFromJSON recognizes this type
	_prm, err := newPolicyReferenceMatchFromJSON(validJSON)
	require.NoError(t, err)
	assert.Equal(t, validPRM, _prm)

	// Various ways to corrupt the JSON
	breakFns := []func(mSI){
		// The "type" field is missing
		func(v mSI) { delete(v, "type") },
		// Wrong "type" field
		func(v mSI) { v["type"] = 1 },
		func(v mSI) { v["type"] = "this is invalid" },
		// Extra top-level sub-object
		func(v mSI) { v["unexpected"] = 1 },
		// The "dockerRepository" field is missing
		func(v mSI) { delete(v, "dockerRepository") },
		// Invalid "dockerRepository" field
		func(v mSI) { v["dockerRepository"] = 1 },
	}
	for _, fn := range breakFns {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		fn(tmp)

		testJSON, err := json.Marshal(tmp)
		require.NoError(t, err)

		prm = prmExactRepository{}
		err = json.Unmarshal(testJSON, &prm)
		assert.Error(t, err)
	}

	// Duplicated fields
	for _, field := range []string{"type", "baseLayerIdentity"} {
		var tmp mSI
		err := json.Unmarshal(validJSON, &tmp)
		require.NoError(t, err)

		testJSON := addExtraJSONMember(t, validJSON, field, tmp[field])

		prm = prmExactRepository{}
		err = json.Unmarshal(testJSON, &prm)
		assert.Error(t, err)
	}
}
