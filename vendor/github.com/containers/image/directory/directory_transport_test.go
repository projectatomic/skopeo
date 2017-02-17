package directory

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
	assert.Equal(t, "dir", Transport.Name())
}

func TestTransportParseReference(t *testing.T) {
	testNewReference(t, Transport.ParseReference)
}

func TestTransportValidatePolicyConfigurationScope(t *testing.T) {
	for _, scope := range []string{
		"/etc",
		"/this/does/not/exist",
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.NoError(t, err, scope)
	}

	for _, scope := range []string{
		"relative/path",
		"/double//slashes",
		"/has/./dot",
		"/has/dot/../dot",
		"/trailing/slash/",
		"/",
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.Error(t, err, scope)
	}
}

func TestNewReference(t *testing.T) {
	testNewReference(t, NewReference)
}

// testNewReference is a test shared for Transport.ParseReference and NewReference.
func testNewReference(t *testing.T, fn func(string) (types.ImageReference, error)) {
	tmpDir, err := ioutil.TempDir("", "dir-transport-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	for _, path := range []string{
		"/",
		"/etc",
		tmpDir,
		"relativepath",
		tmpDir + "/thisdoesnotexist",
	} {
		ref, err := fn(path)
		require.NoError(t, err, path)
		dirRef, ok := ref.(dirReference)
		require.True(t, ok)
		assert.Equal(t, path, dirRef.path, path)
	}

	_, err = fn(tmpDir + "/thisparentdoesnotexist/something")
	assert.Error(t, err)
}

// refToTempDir creates a temporary directory and returns a reference to it.
// The caller should
//   defer os.RemoveAll(tmpDir)
func refToTempDir(t *testing.T) (ref types.ImageReference, tmpDir string) {
	tmpDir, err := ioutil.TempDir("", "dir-transport-test")
	require.NoError(t, err)
	ref, err = NewReference(tmpDir)
	require.NoError(t, err)
	return ref, tmpDir
}

func TestReferenceTransport(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)
	assert.Equal(t, Transport, ref.Transport())
}

func TestReferenceStringWithinTransport(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)
	assert.Equal(t, tmpDir, ref.StringWithinTransport())
}

func TestReferenceDockerReference(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)
	assert.Nil(t, ref.DockerReference())
}

func TestReferencePolicyConfigurationIdentity(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)

	assert.Equal(t, tmpDir, ref.PolicyConfigurationIdentity())
	// A non-canonical path.  Test just one, the various other cases are
	// tested in explicitfilepath.ResolvePathToFullyExplicit.
	ref, err := NewReference(tmpDir + "/.")
	require.NoError(t, err)
	assert.Equal(t, tmpDir, ref.PolicyConfigurationIdentity())

	// "/" as a corner case.
	ref, err = NewReference("/")
	require.NoError(t, err)
	assert.Equal(t, "/", ref.PolicyConfigurationIdentity())
}

func TestReferencePolicyConfigurationNamespaces(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)
	// We don't really know enough to make a full equality test here.
	ns := ref.PolicyConfigurationNamespaces()
	require.NotNil(t, ns)
	assert.NotEmpty(t, ns)
	assert.Equal(t, filepath.Dir(tmpDir), ns[0])

	// Test with a known path which should exist. Test just one non-canonical
	// path, the various other cases are tested in explicitfilepath.ResolvePathToFullyExplicit.
	//
	// It would be nice to test a deeper hierarchy, but it is not obvious what
	// deeper path is always available in the various distros, AND is not likely
	// to contains a symbolic link.
	for _, path := range []string{"/etc/skel", "/etc/skel/./."} {
		_, err := os.Lstat(path)
		require.NoError(t, err)
		ref, err := NewReference(path)
		require.NoError(t, err)
		ns := ref.PolicyConfigurationNamespaces()
		require.NotNil(t, ns)
		assert.Equal(t, []string{"/etc"}, ns)
	}

	// "/" as a corner case.
	ref, err := NewReference("/")
	require.NoError(t, err)
	assert.Equal(t, []string{}, ref.PolicyConfigurationNamespaces())
}

func TestReferenceNewImage(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)

	dest, err := ref.NewImageDestination(nil)
	require.NoError(t, err)
	defer dest.Close()
	mFixture, err := ioutil.ReadFile("../manifest/fixtures/v2s1.manifest.json")
	require.NoError(t, err)
	err = dest.PutManifest(mFixture)
	assert.NoError(t, err)
	err = dest.Commit()
	assert.NoError(t, err)

	img, err := ref.NewImage(nil)
	assert.NoError(t, err)
	defer img.Close()
}

func TestReferenceNewImageNoValidManifest(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)

	dest, err := ref.NewImageDestination(nil)
	require.NoError(t, err)
	defer dest.Close()
	err = dest.PutManifest([]byte(`{"schemaVersion":1}`))
	assert.NoError(t, err)
	err = dest.Commit()
	assert.NoError(t, err)

	_, err = ref.NewImage(nil)
	assert.Error(t, err)
}

func TestReferenceNewImageSource(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)
	src, err := ref.NewImageSource(nil, nil)
	assert.NoError(t, err)
	defer src.Close()
}

func TestReferenceNewImageDestination(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)
	dest, err := ref.NewImageDestination(nil)
	assert.NoError(t, err)
	defer dest.Close()
}

func TestReferenceDeleteImage(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)
	err := ref.DeleteImage(nil)
	assert.Error(t, err)
}

func TestReferenceManifestPath(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)
	dirRef, ok := ref.(dirReference)
	require.True(t, ok)
	assert.Equal(t, tmpDir+"/manifest.json", dirRef.manifestPath())
}

func TestReferenceLayerPath(t *testing.T) {
	const hex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)
	dirRef, ok := ref.(dirReference)
	require.True(t, ok)
	assert.Equal(t, tmpDir+"/"+hex+".tar", dirRef.layerPath("sha256:"+hex))
}

func TestReferenceSignaturePath(t *testing.T) {
	ref, tmpDir := refToTempDir(t)
	defer os.RemoveAll(tmpDir)
	dirRef, ok := ref.(dirReference)
	require.True(t, ok)
	assert.Equal(t, tmpDir+"/signature-1", dirRef.signaturePath(0))
	assert.Equal(t, tmpDir+"/signature-10", dirRef.signaturePath(9))
}
