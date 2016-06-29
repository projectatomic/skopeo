package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-check/check"
	"github.com/projectatomic/skopeo/manifest"
)

func init() {
	check.Suite(&CopySuite{})
}

type CopySuite struct {
	cluster *openshiftCluster
}

func (s *CopySuite) SetUpSuite(c *check.C) {
	if os.Getenv("SKOPEO_CONTAINER_TESTS") != "1" {
		c.Skip("Not running in a container, refusing to affect user state")
	}

	s.cluster = startOpenshiftCluster(c)

	for _, stream := range []string{"unsigned"} {
		isJSON := fmt.Sprintf(`{
			"kind": "ImageStream",
			"apiVersion": "v1",
			"metadata": {
			    "name": "%s"
			},
			"spec": {}
		}`, stream)
		runCommandWithInput(c, isJSON, "oc", "create", "-f", "-")
	}
}

func (s *CopySuite) TearDownSuite(c *check.C) {
	if s.cluster != nil {
		s.cluster.tearDown()
	}
}

// The most basic (skopeo copy) use:
func (s *CopySuite) TestCopySimple(c *check.C) {
	dir1, err := ioutil.TempDir("", "copy-1")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir1)
	dir2, err := ioutil.TempDir("", "copy-2")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir2)

	// FIXME: It would be nice to use one of the local Docker registries instead of neeeding an Internet connection.
	// "pull": docker: → dir:
	assertSkopeoSucceeds(c, "", "copy", "docker://busybox:latest", "dir:"+dir1)
	// "push": dir: → atomic:
	assertSkopeoSucceeds(c, "", "--debug", "copy", "dir:"+dir1, "atomic:myns/unsigned:unsigned")
	// The result of pushing and pulling is an unmodified image.
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/unsigned:unsigned", "dir:"+dir2)
	out := combinedOutputOfCommand(c, "diff", "-urN", dir1, dir2)
	c.Assert(out, check.Equals, "")

	// FIXME: Also check pushing to docker://
}

// Streaming (skopeo copy)
func (s *CopySuite) TestCopyStreaming(c *check.C) {
	dir1, err := ioutil.TempDir("", "streaming-1")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir1)
	dir2, err := ioutil.TempDir("", "streaming-2")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir2)

	// FIXME: It would be nice to use one of the local Docker registries instead of neeeding an Internet connection.
	// streaming: docker: → atomic:
	assertSkopeoSucceeds(c, "", "--debug", "copy", "docker://busybox:1-glibc", "atomic:myns/unsigned:streaming")
	// Compare (copies of) the original and the copy:
	assertSkopeoSucceeds(c, "", "copy", "docker://busybox:1-glibc", "dir:"+dir1)
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/unsigned:streaming", "dir:"+dir2)
	// The manifests will have different JWS signatures; so, compare the manifests by digests, which
	// strips the signatures, and remove them, comparing the rest file by file.
	digests := []string{}
	for _, dir := range []string{dir1, dir2} {
		manifestPath := filepath.Join(dir, "manifest.json")
		m, err := ioutil.ReadFile(manifestPath)
		c.Assert(err, check.IsNil)
		digest, err := manifest.Digest(m)
		c.Assert(err, check.IsNil)
		digests = append(digests, digest)
		err = os.Remove(manifestPath)
		c.Assert(err, check.IsNil)
		c.Logf("Manifest file %s (digest %s) removed", manifestPath, digest)
	}
	c.Assert(digests[0], check.Equals, digests[1])
	out := combinedOutputOfCommand(c, "diff", "-urN", dir1, dir2)
	c.Assert(out, check.Equals, "")
	// FIXME: Also check pushing to docker://
}
