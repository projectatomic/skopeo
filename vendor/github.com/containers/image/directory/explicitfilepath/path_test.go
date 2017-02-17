package explicitfilepath

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pathResolvingTestCase struct {
	setup    func(*testing.T, string) string
	expected string
}

var testCases = []pathResolvingTestCase{
	{ // A straightforward subdirectory hierarchy
		func(t *testing.T, top string) string {
			err := os.MkdirAll(filepath.Join(top, "dir1/dir2/dir3"), 0755)
			require.NoError(t, err)
			return "dir1/dir2/dir3"
		},
		"dir1/dir2/dir3",
	},
	{ // Missing component
		func(t *testing.T, top string) string {
			return "thisismissing/dir2"
		},
		"",
	},
	{ // Symlink on the path
		func(t *testing.T, top string) string {
			err := os.MkdirAll(filepath.Join(top, "dir1/dir2"), 0755)
			require.NoError(t, err)
			err = os.Symlink("dir1", filepath.Join(top, "link1"))
			require.NoError(t, err)
			return "link1/dir2"
		},
		"dir1/dir2",
	},
	{ // Trailing symlink
		func(t *testing.T, top string) string {
			err := os.MkdirAll(filepath.Join(top, "dir1/dir2"), 0755)
			require.NoError(t, err)
			err = os.Symlink("dir2", filepath.Join(top, "dir1/link2"))
			require.NoError(t, err)
			return "dir1/link2"
		},
		"dir1/dir2",
	},
	{ // Symlink pointing nowhere, as a non-final component
		func(t *testing.T, top string) string {
			err := os.Symlink("thisismissing", filepath.Join(top, "link1"))
			require.NoError(t, err)
			return "link1/dir2"
		},
		"",
	},
	{ // Trailing symlink pointing nowhere (but note that a missing non-symlink would be accepted)
		func(t *testing.T, top string) string {
			err := os.Symlink("thisismissing", filepath.Join(top, "link1"))
			require.NoError(t, err)
			return "link1"
		},
		"",
	},
	{ // Relative components in a path
		func(t *testing.T, top string) string {
			err := os.MkdirAll(filepath.Join(top, "dir1/dir2/dir3"), 0755)
			require.NoError(t, err)
			return "dir1/./dir2/../dir2/dir3"
		},
		"dir1/dir2/dir3",
	},
	{ // Trailing relative components
		func(t *testing.T, top string) string {
			err := os.MkdirAll(filepath.Join(top, "dir1/dir2"), 0755)
			require.NoError(t, err)
			return "dir1/dir2/.."
		},
		"dir1",
	},
	{ // Relative components in symlink
		func(t *testing.T, top string) string {
			err := os.MkdirAll(filepath.Join(top, "dir1/dir2"), 0755)
			require.NoError(t, err)
			err = os.Symlink("../dir1/dir2", filepath.Join(top, "dir1/link2"))
			require.NoError(t, err)
			return "dir1/link2"
		},
		"dir1/dir2",
	},
	{ // Relative component pointing "into" a symlink
		func(t *testing.T, top string) string {
			err := os.MkdirAll(filepath.Join(top, "dir1/dir2/dir3"), 0755)
			require.NoError(t, err)
			err = os.Symlink("dir3", filepath.Join(top, "dir1/dir2/link3"))
			require.NoError(t, err)
			return "dir1/dir2/link3/../.."
		},
		"dir1",
	},
	{ // Unreadable directory
		func(t *testing.T, top string) string {
			err := os.MkdirAll(filepath.Join(top, "unreadable/dir2"), 0755)
			require.NoError(t, err)
			err = os.Chmod(filepath.Join(top, "unreadable"), 000)
			require.NoError(t, err)
			return "unreadable/dir2"
		},
		"",
	},
}

func testPathsAreSameFile(t *testing.T, path1, path2, description string) {
	fi1, err := os.Stat(path1)
	require.NoError(t, err)
	fi2, err := os.Stat(path2)
	require.NoError(t, err)
	assert.True(t, os.SameFile(fi1, fi2), description)
}

func runPathResolvingTestCase(t *testing.T, f func(string) (string, error), c pathResolvingTestCase, suffix string) {
	topDir, err := ioutil.TempDir("", "pathResolving")
	defer func() {
		// Clean up after the "Unreadable directory" case; os.RemoveAll just fails.
		_ = os.Chmod(filepath.Join(topDir, "unreadable"), 0755) // Ignore errors, especially if this does not exist.
		os.RemoveAll(topDir)
	}()

	input := c.setup(t, topDir) + suffix // Do not call filepath.Join() on input, it calls filepath.Clean() internally!
	description := fmt.Sprintf("%s vs. %s%s", input, c.expected, suffix)

	fullOutput, err := ResolvePathToFullyExplicit(topDir + "/" + input)
	if c.expected == "" {
		assert.Error(t, err, description)
	} else {
		require.NoError(t, err, input)
		fullExpected := topDir + "/" + c.expected + suffix
		assert.Equal(t, fullExpected, fullOutput)

		// Either the two paths resolve to the same existing file, or to the same name in the same existing parent.
		if _, err := os.Lstat(fullExpected); err == nil {
			testPathsAreSameFile(t, fullOutput, fullExpected, description)
		} else {
			require.True(t, os.IsNotExist(err))
			_, err := os.Stat(fullOutput)
			require.Error(t, err)
			require.True(t, os.IsNotExist(err))

			parentExpected, fileExpected := filepath.Split(fullExpected)
			parentOutput, fileOutput := filepath.Split(fullOutput)
			assert.Equal(t, fileExpected, fileOutput)
			testPathsAreSameFile(t, parentOutput, parentExpected, description)
		}
	}
}

func TestResolvePathToFullyExplicit(t *testing.T) {
	for _, c := range testCases {
		runPathResolvingTestCase(t, ResolvePathToFullyExplicit, c, "")
		runPathResolvingTestCase(t, ResolvePathToFullyExplicit, c, "/trailing")
	}
}

func TestResolveExistingPathToFullyExplicit(t *testing.T) {
	for _, c := range testCases {
		runPathResolvingTestCase(t, resolveExistingPathToFullyExplicit, c, "")
	}
}
