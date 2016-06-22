package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-check/check"
	"github.com/projectatomic/skopeo/docker/utils"
)

func init() {
	check.Suite(&CopySuite{})
}

type CopySuite struct {
	cluster *openshiftCluster
	gpgHome string
}

func (s *CopySuite) SetUpSuite(c *check.C) {
	if os.Getenv("SKOPEO_CONTAINER_TESTS") != "1" {
		c.Skip("Not running in a container, refusing to affect user state")
	}

	s.cluster = startOpenshiftCluster(c)

	for _, stream := range []string{"unsigned", "personal", "official", "naming", "cosigned"} {
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

	gpgHome, err := ioutil.TempDir("", "skopeo-gpg")
	c.Assert(err, check.IsNil)
	s.gpgHome = gpgHome
	os.Setenv("GNUPGHOME", s.gpgHome)

	for _, key := range []string{"personal", "official"} {
		batchInput := fmt.Sprintf("Key-Type: RSA\nName-Real: Test key - %s\nName-email: %s@example.com\n%%commit\n",
			key, key)
		runCommandWithInput(c, batchInput, gpgBinary, "--batch", "--gen-key")

		out := combinedOutputOfCommand(c, gpgBinary, "--armor", "--export", fmt.Sprintf("%s@example.com", key))
		err := ioutil.WriteFile(filepath.Join(s.gpgHome, fmt.Sprintf("%s-pubkey.gpg", key)),
			[]byte(out), 0600)
		c.Assert(err, check.IsNil)
	}
}

func (s *CopySuite) TearDownSuite(c *check.C) {
	if s.gpgHome != "" {
		os.RemoveAll(s.gpgHome)
	}
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
		manifest, err := ioutil.ReadFile(manifestPath)
		c.Assert(err, check.IsNil)
		digest, err := utils.ManifestDigest(manifest)
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

// --sign-by and --policy copy
func (s *CopySuite) TestCopySignatures(c *check.C) {
	dir, err := ioutil.TempDir("", "signatures-dest")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir)
	dirDest := "dir:" + dir

	policyFile, err := ioutil.TempFile("", "policy.json")
	c.Assert(err, check.IsNil)
	policy := policyFile.Name()
	defer os.Remove(policy)
	policyJSON := combinedOutputOfCommand(c, "sed", "s,@keydir@,"+s.gpgHome+",g", "fixtures/policy.json")
	_, err = policyFile.Write([]byte(policyJSON))
	c.Assert(err, check.IsNil)
	err = policyFile.Close()
	c.Assert(err, check.IsNil)

	// type: reject
	assertSkopeoFails(c, ".*Source image rejected: Running these images is rejected by policy.*",
		"--policy", policy, "copy", "docker://busybox:latest", dirDest)

	// type: insecureAcceptAnything
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "docker://openshift/hello-openshift", dirDest)

	// type: signedBy
	// Sign the images
	assertSkopeoSucceeds(c, "", "copy", "--sign-by", "personal@example.com", "docker://busybox:1.23", "atomic:myns/personal:personal")
	assertSkopeoSucceeds(c, "", "copy", "--sign-by", "official@example.com", "docker://busybox:1.23.2", "atomic:myns/official:official")
	// Verify that we can pull them
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "atomic:myns/personal:personal", dirDest)
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "atomic:myns/official:official", dirDest)
	// Verify that mis-signed images are rejected
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/personal:personal", "atomic:myns/official:attack")
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/official:official", "atomic:myns/personal:attack")
	assertSkopeoFails(c, ".*Source image rejected: Invalid GPG signature.*",
		"--policy", policy, "copy", "atomic:myns/personal:attack", dirDest)
	assertSkopeoFails(c, ".*Source image rejected: Invalid GPG signature.*",
		"--policy", policy, "copy", "atomic:myns/official:attack", dirDest)

	// Verify that signed identity is verified.
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/official:official", "atomic:myns/naming:test1")
	assertSkopeoFails(c, ".*Source image rejected: Signature for identity localhost:8443/myns/official:official is not accepted.*",
		"--policy", policy, "copy", "atomic:myns/naming:test1", dirDest)
	// signedIdentity works
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/official:official", "atomic:myns/naming:naming")
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "atomic:myns/naming:naming", dirDest)

	// Verify that cosigning requirements are enforced
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/official:official", "atomic:myns/cosigned:cosigned")
	assertSkopeoFails(c, ".*Source image rejected: Invalid GPG signature.*",
		"--policy", policy, "copy", "atomic:myns/cosigned:cosigned", dirDest)

	assertSkopeoSucceeds(c, "", "copy", "--sign-by", "personal@example.com", "atomic:myns/official:official", "atomic:myns/cosigned:cosigned")
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "atomic:myns/cosigned:cosigned", dirDest)
}
