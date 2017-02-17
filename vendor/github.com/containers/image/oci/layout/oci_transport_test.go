package layout

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransportName(t *testing.T) {
	assert.Equal(t, "oci", Transport.Name())
}

func TestTransportParseReference(t *testing.T) {
	testParseReference(t, Transport.ParseReference)
}

func TestTransportValidatePolicyConfigurationScope(t *testing.T) {
	for _, scope := range []string{
		"/etc",
		"/etc:notlatest",
		"/this/does/not/exist",
		"/this/does/not/exist:notlatest",
		"/:strangecornercase",
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.NoError(t, err, scope)
	}

	for _, scope := range []string{
		"relative/path",
		"/",
		"/double//slashes",
		"/has/./dot",
		"/has/dot/../dot",
		"/trailing/slash/",
		"/etc:invalid'tag!value@",
		"/path:with/colons",
		"/path:with/colons/and:tag",
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
	tmpDir, err := ioutil.TempDir("", "oci-transport-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	for _, path := range []string{
		"/",
		"/etc",
		tmpDir,
		"relativepath",
		tmpDir + "/thisdoesnotexist",
	} {
		for _, tag := range []struct{ suffix, tag string }{
			{":notlatest", "notlatest"},
			{"", "latest"},
		} {
			input := path + tag.suffix
			ref, err := fn(input)
			require.NoError(t, err, input)
			ociRef, ok := ref.(ociReference)
			require.True(t, ok)
			assert.Equal(t, path, ociRef.dir, input)
			assert.Equal(t, tag.tag, ociRef.tag, input)
		}
	}

	_, err = fn(tmpDir + "/with:multiple:colons:and:tag")
	assert.Error(t, err)

	_, err = fn(tmpDir + ":invalid'tag!value@")
	assert.Error(t, err)
}

func TestNewReference(t *testing.T) {
	const tagValue = "tagValue"

	tmpDir, err := ioutil.TempDir("", "oci-transport-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ref, err := NewReference(tmpDir, tagValue)
	require.NoError(t, err)
	ociRef, ok := ref.(ociReference)
	require.True(t, ok)
	assert.Equal(t, tmpDir, ociRef.dir)
	assert.Equal(t, tagValue, ociRef.tag)

	_, err = NewReference(tmpDir+"/thisparentdoesnotexist/something", tagValue)
	assert.Error(t, err)

	_, err = NewReference(tmpDir+"/has:colon", tagValue)
	assert.Error(t, err)

	_, err = NewReference(tmpDir, "invalid'tag!value@")
	assert.Error(t, err)
}

// refToTempOCI creates a temporary directory and returns an reference to it.
// The caller should
//   defer os.RemoveAll(tmpDir)
func refToTempOCI(t *testing.T) (ref types.ImageReference, tmpDir string) {
	tmpDir, err := ioutil.TempDir("", "oci-transport-test")
	require.NoError(t, err)
	ref, err = NewReference(tmpDir, "tagValue")
	require.NoError(t, err)
	return ref, tmpDir
}

func TestReferenceTransport(t *testing.T) {
	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)
	assert.Equal(t, Transport, ref.Transport())
}

func TestReferenceStringWithinTransport(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "oci-transport-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	for _, c := range []struct{ input, result string }{
		{"/dir1:notlatest", "/dir1:notlatest"}, // Explicit tag
		{"/dir2", "/dir2:latest"},              // Default tag
	} {
		ref, err := ParseReference(tmpDir + c.input)
		require.NoError(t, err, c.input)
		stringRef := ref.StringWithinTransport()
		assert.Equal(t, tmpDir+c.result, stringRef, c.input)
		// Do one more round to verify that the output can be parsed, to an equal value.
		ref2, err := Transport.ParseReference(stringRef)
		require.NoError(t, err, c.input)
		stringRef2 := ref2.StringWithinTransport()
		assert.Equal(t, stringRef, stringRef2, c.input)
	}
}

func TestReferenceDockerReference(t *testing.T) {
	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)
	assert.Nil(t, ref.DockerReference())
}

func TestReferencePolicyConfigurationIdentity(t *testing.T) {
	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)

	assert.Equal(t, tmpDir+":tagValue", ref.PolicyConfigurationIdentity())
	// A non-canonical path.  Test just one, the various other cases are
	// tested in explicitfilepath.ResolvePathToFullyExplicit.
	ref, err := NewReference(tmpDir+"/.", "tag2")
	require.NoError(t, err)
	assert.Equal(t, tmpDir+":tag2", ref.PolicyConfigurationIdentity())

	// "/" as a corner case.
	ref, err = NewReference("/", "tag3")
	require.NoError(t, err)
	assert.Equal(t, "/:tag3", ref.PolicyConfigurationIdentity())
}

func TestReferencePolicyConfigurationNamespaces(t *testing.T) {
	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)
	// We don't really know enough to make a full equality test here.
	ns := ref.PolicyConfigurationNamespaces()
	require.NotNil(t, ns)
	assert.True(t, len(ns) >= 2)
	assert.Equal(t, tmpDir, ns[0])
	assert.Equal(t, filepath.Dir(tmpDir), ns[1])

	// Test with a known path which should exist. Test just one non-canonical
	// path, the various other cases are tested in explicitfilepath.ResolvePathToFullyExplicit.
	//
	// It would be nice to test a deeper hierarchy, but it is not obvious what
	// deeper path is always available in the various distros, AND is not likely
	// to contains a symbolic link.
	for _, path := range []string{"/etc/skel", "/etc/skel/./."} {
		_, err := os.Lstat(path)
		require.NoError(t, err)
		ref, err := NewReference(path, "sometag")
		require.NoError(t, err)
		ns := ref.PolicyConfigurationNamespaces()
		require.NotNil(t, ns)
		assert.Equal(t, []string{"/etc/skel", "/etc"}, ns)
	}

	// "/" as a corner case.
	ref, err := NewReference("/", "tag3")
	require.NoError(t, err)
	assert.Equal(t, []string{}, ref.PolicyConfigurationNamespaces())
}

func TestReferenceNewImage(t *testing.T) {
	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)
	_, err := ref.NewImage(nil)
	assert.Error(t, err)
}

func TestReferenceNewImageSource(t *testing.T) {
	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)
	_, err := ref.NewImageSource(nil, nil)
	assert.NoError(t, err)
}

func TestReferenceNewImageDestination(t *testing.T) {
	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)
	dest, err := ref.NewImageDestination(nil)
	assert.NoError(t, err)
	defer dest.Close()
}

func TestReferenceDeleteImage(t *testing.T) {
	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)
	err := ref.DeleteImage(nil)
	assert.Error(t, err)
}

func TestReferenceOCILayoutPath(t *testing.T) {
	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)
	ociRef, ok := ref.(ociReference)
	require.True(t, ok)
	assert.Equal(t, tmpDir+"/oci-layout", ociRef.ociLayoutPath())
}

func TestReferenceBlobPath(t *testing.T) {
	const hex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)
	ociRef, ok := ref.(ociReference)
	require.True(t, ok)
	bp, err := ociRef.blobPath("sha256:" + hex)
	assert.NoError(t, err)
	assert.Equal(t, tmpDir+"/blobs/sha256/"+hex, bp)
}

func TestReferenceBlobPathInvalid(t *testing.T) {
	const hex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)
	ociRef, ok := ref.(ociReference)
	require.True(t, ok)
	_, err := ociRef.blobPath(hex)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected digest reference "+hex)
}

func TestReferenceDescriptorPath(t *testing.T) {
	ref, tmpDir := refToTempOCI(t)
	defer os.RemoveAll(tmpDir)
	ociRef, ok := ref.(ociReference)
	require.True(t, ok)
	assert.Equal(t, tmpDir+"/refs/notlatest", ociRef.descriptorPath("notlatest"))
}
