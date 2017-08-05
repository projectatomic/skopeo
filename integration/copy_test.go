package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/manifest"
	"github.com/containers/image/signature"
	"github.com/go-check/check"
	"github.com/opencontainers/go-digest"
)

func init() {
	check.Suite(&CopySuite{})
}

const (
	v2DockerRegistryURL   = "localhost:5555" // Update also policy.json
	v2s1DockerRegistryURL = "localhost:5556"
)

type CopySuite struct {
	cluster    *openshiftCluster
	registry   *testRegistryV2
	s1Registry *testRegistryV2
	gpgHome    string
}

func (s *CopySuite) SetUpSuite(c *check.C) {
	if os.Getenv("SKOPEO_CONTAINER_TESTS") != "1" {
		c.Skip("Not running in a container, refusing to affect user state")
	}

	s.cluster = startOpenshiftCluster(c) // FIXME: Set up TLS for the docker registry port instead of using "--tls-verify=false" all over the place.

	for _, stream := range []string{"unsigned", "personal", "official", "naming", "cosigned", "compression", "schema1", "schema2"} {
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

	// FIXME: Set up TLS for the docker registry port instead of using "--tls-verify=false" all over the place.
	s.registry = setupRegistryV2At(c, v2DockerRegistryURL, false, false)
	s.s1Registry = setupRegistryV2At(c, v2s1DockerRegistryURL, false, true)

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
	if s.registry != nil {
		s.registry.Close()
	}
	if s.s1Registry != nil {
		s.s1Registry.Close()
	}
	if s.cluster != nil {
		s.cluster.tearDown(c)
	}
}

func (s *CopySuite) TestCopyFailsWithManifestList(c *check.C) {
	assertSkopeoFails(c, ".*can not copy docker://estesp/busybox:latest: manifest contains multiple images.*", "copy", "docker://estesp/busybox:latest", "dir:somedir")
}

func (s *CopySuite) TestCopyFailsWhenImageOSDoesntMatchRuntimeOS(c *check.C) {
	c.Skip("can't run this on Travis")
	assertSkopeoFails(c, `.*image operating system "windows" cannot be used on "linux".*`, "copy", "docker://microsoft/windowsservercore", "containers-storage:test")
}

func (s *CopySuite) TestCopySimpleAtomicRegistry(c *check.C) {
	dir1, err := ioutil.TempDir("", "copy-1")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir1)
	dir2, err := ioutil.TempDir("", "copy-2")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir2)

	// FIXME: It would be nice to use one of the local Docker registries instead of neeeding an Internet connection.
	// "pull": docker: → dir:
	assertSkopeoSucceeds(c, "", "copy", "docker://estesp/busybox:amd64", "dir:"+dir1)
	// "push": dir: → atomic:
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--debug", "copy", "dir:"+dir1, "atomic:localhost:5000/myns/unsigned:unsigned")
	// The result of pushing and pulling is an equivalent image, except for schema1 embedded names.
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "atomic:localhost:5000/myns/unsigned:unsigned", "dir:"+dir2)
	assertSchema1DirImagesAreEqualExceptNames(c, dir1, "estesp/busybox:amd64", dir2, "myns/unsigned:unsigned")
}

// The most basic (skopeo copy) use:
func (s *CopySuite) TestCopySimple(c *check.C) {
	const ourRegistry = "docker://" + v2DockerRegistryURL + "/"

	dir1, err := ioutil.TempDir("", "copy-1")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir1)
	dir2, err := ioutil.TempDir("", "copy-2")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir2)

	// FIXME: It would be nice to use one of the local Docker registries instead of neeeding an Internet connection.
	// "pull": docker: → dir:
	assertSkopeoSucceeds(c, "", "copy", "docker://busybox", "dir:"+dir1)
	// "push": dir: → docker(v2s2):
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--debug", "copy", "dir:"+dir1, ourRegistry+"busybox:unsigned")
	// The result of pushing and pulling is an unmodified image.
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", ourRegistry+"busybox:unsigned", "dir:"+dir2)
	out := combinedOutputOfCommand(c, "diff", "-urN", dir1, dir2)
	c.Assert(out, check.Equals, "")

	// docker v2s2 -> OCI image layout
	// ociDest will be created by oci: if it doesn't exist
	// so don't create it here to exercise auto-creation
	ociDest := "busybox-latest"
	defer os.RemoveAll(ociDest)
	assertSkopeoSucceeds(c, "", "copy", "docker://busybox:latest", "oci:"+ociDest)
	_, err = os.Stat(ociDest)
	c.Assert(err, check.IsNil)
}

// Check whether dir: images in dir1 and dir2 are equal, ignoring schema1 signatures.
func assertDirImagesAreEqual(c *check.C, dir1, dir2 string) {
	// The manifests may have different JWS signatures; so, compare the manifests by digests, which
	// strips the signatures.
	digests := []digest.Digest{}
	for _, dir := range []string{dir1, dir2} {
		manifestPath := filepath.Join(dir, "manifest.json")
		m, err := ioutil.ReadFile(manifestPath)
		c.Assert(err, check.IsNil)
		digest, err := manifest.Digest(m)
		c.Assert(err, check.IsNil)
		digests = append(digests, digest)
	}
	c.Assert(digests[0], check.Equals, digests[1])
	// Then compare the rest file by file.
	out := combinedOutputOfCommand(c, "diff", "-urN", "-x", "manifest.json", dir1, dir2)
	c.Assert(out, check.Equals, "")
}

// Check whether schema1 dir: images in dir1 and dir2 are equal, ignoring schema1 signatures and the embedded path/tag values, which should have the expected values.
func assertSchema1DirImagesAreEqualExceptNames(c *check.C, dir1, ref1, dir2, ref2 string) {
	// The manifests may have different JWS signatures and names; so, unmarshal and delete these elements.
	manifests := []map[string]interface{}{}
	for dir, ref := range map[string]string{dir1: ref1, dir2: ref2} {
		manifestPath := filepath.Join(dir, "manifest.json")
		m, err := ioutil.ReadFile(manifestPath)
		c.Assert(err, check.IsNil)
		data := map[string]interface{}{}
		err = json.Unmarshal(m, &data)
		c.Assert(err, check.IsNil)
		c.Assert(data["schemaVersion"], check.Equals, float64(1))
		colon := strings.LastIndex(ref, ":")
		c.Assert(colon, check.Not(check.Equals), -1)
		c.Assert(data["name"], check.Equals, ref[:colon])
		c.Assert(data["tag"], check.Equals, ref[colon+1:])
		for _, key := range []string{"signatures", "name", "tag"} {
			delete(data, key)
		}
		manifests = append(manifests, data)
	}
	c.Assert(manifests[0], check.DeepEquals, manifests[1])
	// Then compare the rest file by file.
	out := combinedOutputOfCommand(c, "diff", "-urN", "-x", "manifest.json", dir1, dir2)
	c.Assert(out, check.Equals, "")
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
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--debug", "copy", "docker://estesp/busybox:amd64", "atomic:localhost:5000/myns/unsigned:streaming")
	// Compare (copies of) the original and the copy:
	assertSkopeoSucceeds(c, "", "copy", "docker://estesp/busybox:amd64", "dir:"+dir1)
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "atomic:localhost:5000/myns/unsigned:streaming", "dir:"+dir2)
	assertSchema1DirImagesAreEqualExceptNames(c, dir1, "estesp/busybox:amd64", dir2, "myns/unsigned:streaming")
	// FIXME: Also check pushing to docker://
}

// OCI round-trip testing. It's very important to make sure that OCI <-> Docker
// conversion works (while skopeo handles many things, one of the most obvious
// benefits of a tool like skopeo is that you can use OCI tooling to create an
// image and then as the final step convert the image to a non-standard format
// like Docker). But this only works if we _test_ it.
func (s *CopySuite) TestCopyOCIRoundTrip(c *check.C) {
	const ourRegistry = "docker://" + v2DockerRegistryURL + "/"

	oci1, err := ioutil.TempDir("", "oci-1")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(oci1)
	oci2, err := ioutil.TempDir("", "oci-2")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(oci2)

	// Docker -> OCI
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--debug", "copy", "docker://busybox", "oci:"+oci1+":latest")
	// OCI -> Docker
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--debug", "copy", "oci:"+oci1+":latest", ourRegistry+"original/busybox:oci_copy")
	// Docker -> OCI
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--debug", "copy", ourRegistry+"original/busybox:oci_copy", "oci:"+oci2+":latest")
	// OCI -> Docker
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--debug", "copy", "oci:"+oci2+":latest", ourRegistry+"original/busybox:oci_copy2")

	// TODO: Add some more tags to output to and check those work properly.

	// First, make sure the OCI blobs are the same. This should _always_ be true.
	out := combinedOutputOfCommand(c, "diff", "-urN", oci1+"/blobs", oci2+"/blobs")
	c.Assert(out, check.Equals, "")

	// For some silly reason we pass a logger to the OCI library here...
	//logger := log.New(os.Stderr, "", 0)

	// TODO: Verify using the upstream OCI image validator.
	//err = image.ValidateLayout(oci1, nil, logger)
	//c.Assert(err, check.IsNil)
	//err = image.ValidateLayout(oci2, nil, logger)
	//c.Assert(err, check.IsNil)

	// Now verify that everything is identical. Currently this is true, but
	// because we recompute the manifests on-the-fly this doesn't necessarily
	// always have to be true (but if this breaks in the future __PLEASE__ make
	// sure that the breakage actually makes sense before removing this check).
	out = combinedOutputOfCommand(c, "diff", "-urN", oci1, oci2)
	c.Assert(out, check.Equals, "")
}

// --sign-by and --policy copy, primarily using atomic:
func (s *CopySuite) TestCopySignatures(c *check.C) {
	mech, _, err := signature.NewEphemeralGPGSigningMechanism([]byte{})
	c.Assert(err, check.IsNil)
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil { // FIXME? Test that verification and policy enforcement works, using signatures from fixtures
		c.Skip(fmt.Sprintf("Signing not supported: %v", err))
	}

	dir, err := ioutil.TempDir("", "signatures-dest")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir)
	dirDest := "dir:" + dir

	policy := fileFromFixture(c, "fixtures/policy.json", map[string]string{"@keydir@": s.gpgHome})
	defer os.Remove(policy)

	// type: reject
	assertSkopeoFails(c, ".*Source image rejected: Running image docker://busybox:latest is rejected by policy.*",
		"--policy", policy, "copy", "docker://busybox:latest", dirDest)

	// type: accept
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "docker://openshift/hello-openshift", dirDest)

	// type: signedBy
	// Sign the images
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "--sign-by", "personal@example.com", "docker://busybox:1.26", "atomic:localhost:5006/myns/personal:personal")
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "--sign-by", "official@example.com", "docker://busybox:1.26.1", "atomic:localhost:5006/myns/official:official")
	// Verify that we can pull them
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--policy", policy, "copy", "atomic:localhost:5006/myns/personal:personal", dirDest)
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--policy", policy, "copy", "atomic:localhost:5006/myns/official:official", dirDest)
	// Verify that mis-signed images are rejected
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "atomic:localhost:5006/myns/personal:personal", "atomic:localhost:5006/myns/official:attack")
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "atomic:localhost:5006/myns/official:official", "atomic:localhost:5006/myns/personal:attack")
	assertSkopeoFails(c, ".*Source image rejected: Invalid GPG signature.*",
		"--tls-verify=false", "--policy", policy, "copy", "atomic:localhost:5006/myns/personal:attack", dirDest)
	assertSkopeoFails(c, ".*Source image rejected: Invalid GPG signature.*",
		"--tls-verify=false", "--policy", policy, "copy", "atomic:localhost:5006/myns/official:attack", dirDest)

	// Verify that signed identity is verified.
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "atomic:localhost:5006/myns/official:official", "atomic:localhost:5006/myns/naming:test1")
	assertSkopeoFails(c, ".*Source image rejected: Signature for identity localhost:5006/myns/official:official is not accepted.*",
		"--tls-verify=false", "--policy", policy, "copy", "atomic:localhost:5006/myns/naming:test1", dirDest)
	// signedIdentity works
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "atomic:localhost:5006/myns/official:official", "atomic:localhost:5006/myns/naming:naming")
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--policy", policy, "copy", "atomic:localhost:5006/myns/naming:naming", dirDest)

	// Verify that cosigning requirements are enforced
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "atomic:localhost:5006/myns/official:official", "atomic:localhost:5006/myns/cosigned:cosigned")
	assertSkopeoFails(c, ".*Source image rejected: Invalid GPG signature.*",
		"--tls-verify=false", "--policy", policy, "copy", "atomic:localhost:5006/myns/cosigned:cosigned", dirDest)

	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "--sign-by", "personal@example.com", "atomic:localhost:5006/myns/official:official", "atomic:localhost:5006/myns/cosigned:cosigned")
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--policy", policy, "copy", "atomic:localhost:5006/myns/cosigned:cosigned", dirDest)
}

// --policy copy for dir: sources
func (s *CopySuite) TestCopyDirSignatures(c *check.C) {
	mech, _, err := signature.NewEphemeralGPGSigningMechanism([]byte{})
	c.Assert(err, check.IsNil)
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil { // FIXME? Test that verification and policy enforcement works, using signatures from fixtures
		c.Skip(fmt.Sprintf("Signing not supported: %v", err))
	}

	topDir, err := ioutil.TempDir("", "dir-signatures-top")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(topDir)
	topDirDest := "dir:" + topDir

	for _, suffix := range []string{"/dir1", "/dir2", "/restricted/personal", "/restricted/official", "/restricted/badidentity", "/dest"} {
		err := os.MkdirAll(topDir+suffix, 0755)
		c.Assert(err, check.IsNil)
	}

	// Note the "/@dirpath@": The value starts with a slash so that it is not rejected in other tests which do not replace it,
	// but we must ensure that the result is a canonical path, not something starting with a "//".
	policy := fileFromFixture(c, "fixtures/policy.json", map[string]string{"@keydir@": s.gpgHome, "/@dirpath@": topDir + "/restricted"})
	defer os.Remove(policy)

	// Get some images.
	assertSkopeoSucceeds(c, "", "copy", "docker://estesp/busybox:armfh", topDirDest+"/dir1")
	assertSkopeoSucceeds(c, "", "copy", "docker://estesp/busybox:s390x", topDirDest+"/dir2")

	// Sign the images. By coping fom a topDirDest/dirN, also test that non-/restricted paths
	// use the dir:"" default of accept.
	// (For signing, we must push to atomic: to get a Docker identity to use in the signature.)
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--policy", policy, "copy", "--sign-by", "personal@example.com", topDirDest+"/dir1", "atomic:localhost:5000/myns/personal:dirstaging")
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--policy", policy, "copy", "--sign-by", "official@example.com", topDirDest+"/dir2", "atomic:localhost:5000/myns/official:dirstaging")
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "atomic:localhost:5000/myns/personal:dirstaging", topDirDest+"/restricted/personal")
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "atomic:localhost:5000/myns/official:dirstaging", topDirDest+"/restricted/official")

	// type: signedBy, with a signedIdentity override (necessary because dir: identities can't be signed)
	// Verify that correct images are accepted
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", topDirDest+"/restricted/official", topDirDest+"/dest")
	// ... and that mis-signed images are rejected.
	assertSkopeoFails(c, ".*Source image rejected: Invalid GPG signature.*",
		"--policy", policy, "copy", topDirDest+"/restricted/personal", topDirDest+"/dest")

	// Verify that the signed identity is verified.
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--policy", policy, "copy", "--sign-by", "official@example.com", topDirDest+"/dir1", "atomic:localhost:5000/myns/personal:dirstaging2")
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "atomic:localhost:5000/myns/personal:dirstaging2", topDirDest+"/restricted/badidentity")
	assertSkopeoFails(c, ".*Source image rejected: .*Signature for identity localhost:5000/myns/personal:dirstaging2 is not accepted.*",
		"--policy", policy, "copy", topDirDest+"/restricted/badidentity", topDirDest+"/dest")
}

// Compression during copy
func (s *CopySuite) TestCopyCompression(c *check.C) {
	const uncompresssedLayerFile = "160d823fdc48e62f97ba62df31e55424f8f5eb6b679c865eec6e59adfe304710.tar"

	topDir, err := ioutil.TempDir("", "compression-top")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(topDir)

	for i, t := range []struct{ fixture, remote string }{
		{"uncompressed-image-s1", "docker://" + v2DockerRegistryURL + "/compression/compression:s1"},
		{"uncompressed-image-s2", "docker://" + v2DockerRegistryURL + "/compression/compression:s2"},
		{"uncompressed-image-s1", "atomic:localhost:5000/myns/compression:s1"},
		{"uncompressed-image-s2", "atomic:localhost:5000/myns/compression:s2"},
	} {
		dir := filepath.Join(topDir, fmt.Sprintf("case%d", i))
		err := os.MkdirAll(dir, 0755)
		c.Assert(err, check.IsNil)

		assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "dir:fixtures/"+t.fixture, t.remote)
		assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", t.remote, "dir:"+dir)

		// The original directory contained an uncompressed file, the copy after pushing and pulling doesn't (we use a different name for the compressed file).
		_, err = os.Lstat(filepath.Join("fixtures", t.fixture, uncompresssedLayerFile))
		c.Assert(err, check.IsNil)
		_, err = os.Lstat(filepath.Join(dir, uncompresssedLayerFile))
		c.Assert(err, check.NotNil)
		c.Assert(os.IsNotExist(err), check.Equals, true)

		// All pulled layers are smaller than the uncompressed size of uncompresssedLayerFile. (Note that this includes the manifest in s2, but that works out OK).
		dirf, err := os.Open(dir)
		c.Assert(err, check.IsNil)
		fis, err := dirf.Readdir(-1)
		c.Assert(err, check.IsNil)
		for _, fi := range fis {
			if strings.HasSuffix(fi.Name(), ".tar") {
				c.Assert(fi.Size() < 2048, check.Equals, true)
			}
		}
	}
}

func findRegularFiles(c *check.C, root string) []string {
	result := []string{}
	err := filepath.Walk(root, filepath.WalkFunc(func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			result = append(result, path)
		}
		return nil
	}))
	c.Assert(err, check.IsNil)
	return result
}

// --sign-by and policy use for docker: with sigstore
func (s *CopySuite) TestCopyDockerSigstore(c *check.C) {
	mech, _, err := signature.NewEphemeralGPGSigningMechanism([]byte{})
	c.Assert(err, check.IsNil)
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil { // FIXME? Test that verification and policy enforcement works, using signatures from fixtures
		c.Skip(fmt.Sprintf("Signing not supported: %v", err))
	}

	const ourRegistry = "docker://" + v2DockerRegistryURL + "/"

	tmpDir, err := ioutil.TempDir("", "signatures-sigstore")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)
	copyDest := filepath.Join(tmpDir, "dest")
	err = os.Mkdir(copyDest, 0755)
	c.Assert(err, check.IsNil)
	dirDest := "dir:" + copyDest
	plainSigstore := filepath.Join(tmpDir, "sigstore")
	splitSigstoreStaging := filepath.Join(tmpDir, "sigstore-staging")

	splitSigstoreReadServerHandler := http.NotFoundHandler()
	splitSigstoreReadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		splitSigstoreReadServerHandler.ServeHTTP(w, r)
	}))
	defer splitSigstoreReadServer.Close()

	policy := fileFromFixture(c, "fixtures/policy.json", map[string]string{"@keydir@": s.gpgHome})
	defer os.Remove(policy)
	registriesDir := filepath.Join(tmpDir, "registries.d")
	err = os.Mkdir(registriesDir, 0755)
	c.Assert(err, check.IsNil)
	registriesFile := fileFromFixture(c, "fixtures/registries.yaml",
		map[string]string{"@sigstore@": plainSigstore, "@split-staging@": splitSigstoreStaging, "@split-read@": splitSigstoreReadServer.URL})
	err = os.Symlink(registriesFile, filepath.Join(registriesDir, "registries.yaml"))
	c.Assert(err, check.IsNil)

	// Get an image to work with.  Also verifies that we can use Docker repositories with no sigstore configured.
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--registries.d", registriesDir, "copy", "docker://busybox", ourRegistry+"original/busybox")
	// Pulling an unsigned image fails.
	assertSkopeoFails(c, ".*Source image rejected: A signature was required, but no signature exists.*",
		"--tls-verify=false", "--policy", policy, "--registries.d", registriesDir, "copy", ourRegistry+"original/busybox", dirDest)

	// Signing with sigstore defined succeeds,
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--registries.d", registriesDir, "copy", "--sign-by", "personal@example.com", ourRegistry+"original/busybox", ourRegistry+"signed/busybox")
	// a signature file has been created,
	foundFiles := findRegularFiles(c, plainSigstore)
	c.Assert(foundFiles, check.HasLen, 1)
	// and pulling a signed image succeeds.
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--policy", policy, "--registries.d", registriesDir, "copy", ourRegistry+"signed/busybox", dirDest)

	// Deleting the image succeeds,
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--registries.d", registriesDir, "delete", ourRegistry+"signed/busybox")
	// and the signature file has been deleted (but we leave the directories around).
	foundFiles = findRegularFiles(c, plainSigstore)
	c.Assert(foundFiles, check.HasLen, 0)

	// Signing with a read/write sigstore split succeeds,
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--registries.d", registriesDir, "copy", "--sign-by", "personal@example.com", ourRegistry+"original/busybox", ourRegistry+"public/busybox")
	// and a signature file has been created.
	foundFiles = findRegularFiles(c, splitSigstoreStaging)
	c.Assert(foundFiles, check.HasLen, 1)
	// Pulling the image fails because the read sigstore URL has not been populated:
	assertSkopeoFails(c, ".*Source image rejected: A signature was required, but no signature exists.*",
		"--tls-verify=false", "--policy", policy, "--registries.d", registriesDir, "copy", ourRegistry+"public/busybox", dirDest)
	// Pulling the image succeeds after the read sigstore URL is available:
	splitSigstoreReadServerHandler = http.FileServer(http.Dir(splitSigstoreStaging))
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--policy", policy, "--registries.d", registriesDir, "copy", ourRegistry+"public/busybox", dirDest)
}

// atomic: and docker: X-Registry-Supports-Signatures works and interoperates
func (s *CopySuite) TestCopyAtomicExtension(c *check.C) {
	mech, _, err := signature.NewEphemeralGPGSigningMechanism([]byte{})
	c.Assert(err, check.IsNil)
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil { // FIXME? Test that the reading/writing works using signatures from fixtures
		c.Skip(fmt.Sprintf("Signing not supported: %v", err))
	}

	topDir, err := ioutil.TempDir("", "atomic-extension")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(topDir)
	for _, subdir := range []string{"dirAA", "dirAD", "dirDA", "dirDD", "registries.d"} {
		err := os.MkdirAll(filepath.Join(topDir, subdir), 0755)
		c.Assert(err, check.IsNil)
	}
	registriesDir := filepath.Join(topDir, "registries.d")
	dirDest := "dir:" + topDir
	policy := fileFromFixture(c, "fixtures/policy.json", map[string]string{"@keydir@": s.gpgHome})
	defer os.Remove(policy)

	// Get an image to work with to an atomic: destination.  Also verifies that we can use Docker repositories without X-Registry-Supports-Signatures
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--registries.d", registriesDir, "copy", "docker://busybox", "atomic:localhost:5000/myns/extension:unsigned")
	// Pulling an unsigned image using atomic: fails.
	assertSkopeoFails(c, ".*Source image rejected: A signature was required, but no signature exists.*",
		"--tls-verify=false", "--policy", policy,
		"copy", "atomic:localhost:5000/myns/extension:unsigned", dirDest+"/dirAA")
	// The same when pulling using docker:
	assertSkopeoFails(c, ".*Source image rejected: A signature was required, but no signature exists.*",
		"--tls-verify=false", "--policy", policy, "--registries.d", registriesDir,
		"copy", "docker://localhost:5000/myns/extension:unsigned", dirDest+"/dirAD")

	// Sign the image using atomic:
	assertSkopeoSucceeds(c, "", "--tls-verify=false",
		"copy", "--sign-by", "personal@example.com", "atomic:localhost:5000/myns/extension:unsigned", "atomic:localhost:5000/myns/extension:atomic")
	// Pulling the image using atomic: now succeeds.
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--policy", policy,
		"copy", "atomic:localhost:5000/myns/extension:atomic", dirDest+"/dirAA")
	// The same when pulling using docker:
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--policy", policy, "--registries.d", registriesDir,
		"copy", "docker://localhost:5000/myns/extension:atomic", dirDest+"/dirAD")
	// Both access methods result in the same data.
	assertDirImagesAreEqual(c, filepath.Join(topDir, "dirAA"), filepath.Join(topDir, "dirAD"))

	// Get another image (different so that they don't share signatures, and sign it using docker://)
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "--registries.d", registriesDir,
		"copy", "--sign-by", "personal@example.com", "docker://estesp/busybox:ppc64le", "atomic:localhost:5000/myns/extension:extension")
	c.Logf("%s", combinedOutputOfCommand(c, "oc", "get", "istag", "extension:extension", "-o", "json"))
	// Pulling the image using atomic: succeeds.
	assertSkopeoSucceeds(c, "", "--debug", "--tls-verify=false", "--policy", policy,
		"copy", "atomic:localhost:5000/myns/extension:extension", dirDest+"/dirDA")
	// The same when pulling using docker:
	assertSkopeoSucceeds(c, "", "--debug", "--tls-verify=false", "--policy", policy, "--registries.d", registriesDir,
		"copy", "docker://localhost:5000/myns/extension:extension", dirDest+"/dirDD")
	// Both access methods result in the same data.
	assertDirImagesAreEqual(c, filepath.Join(topDir, "dirDA"), filepath.Join(topDir, "dirDD"))
}

func (s *SkopeoSuite) TestCopySrcWithAuth(c *check.C) {
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "--dest-creds=testuser:testpassword", "docker://busybox", fmt.Sprintf("docker://%s/busybox:latest", s.regV2WithAuth.url))
	dir1, err := ioutil.TempDir("", "copy-1")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir1)
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "--src-creds=testuser:testpassword", fmt.Sprintf("docker://%s/busybox:latest", s.regV2WithAuth.url), "dir:"+dir1)
}

func (s *SkopeoSuite) TestCopyDestWithAuth(c *check.C) {
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "--dest-creds=testuser:testpassword", "docker://busybox", fmt.Sprintf("docker://%s/busybox:latest", s.regV2WithAuth.url))
}

func (s *SkopeoSuite) TestCopySrcAndDestWithAuth(c *check.C) {
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "--dest-creds=testuser:testpassword", "docker://busybox", fmt.Sprintf("docker://%s/busybox:latest", s.regV2WithAuth.url))
	assertSkopeoSucceeds(c, "", "--tls-verify=false", "copy", "--src-creds=testuser:testpassword", "--dest-creds=testuser:testpassword", fmt.Sprintf("docker://%s/busybox:latest", s.regV2WithAuth.url), fmt.Sprintf("docker://%s/test:auth", s.regV2WithAuth.url))
}

func (s *CopySuite) TestCopyNoPanicOnHTTPResponseWOTLSVerifyFalse(c *check.C) {
	const ourRegistry = "docker://" + v2DockerRegistryURL + "/"

	// dir:test isn't created beforehand just because we already know this could
	// just fail when evaluating the src
	assertSkopeoFails(c, ".*server gave HTTP response to HTTPS client.*",
		"copy", ourRegistry+"foobar", "dir:test")
}

func (s *CopySuite) TestCopySchemaConversion(c *check.C) {
	// Test conversion / schema autodetection both for the OpenShift embedded registry…
	s.testCopySchemaConversionRegistries(c, "docker://localhost:5005/myns/schema1", "docker://localhost:5006/myns/schema2")
	// … and for various docker/distribution registry versions.
	s.testCopySchemaConversionRegistries(c, "docker://"+v2s1DockerRegistryURL+"/schema1", "docker://"+v2DockerRegistryURL+"/schema2")
}

func (s *CopySuite) testCopySchemaConversionRegistries(c *check.C, schema1Registry, schema2Registry string) {
	topDir, err := ioutil.TempDir("", "schema-conversion")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(topDir)
	for _, subdir := range []string{"input1", "input2", "dest2"} {
		err := os.MkdirAll(filepath.Join(topDir, subdir), 0755)
		c.Assert(err, check.IsNil)
	}
	input1Dir := filepath.Join(topDir, "input1")
	input2Dir := filepath.Join(topDir, "input2")
	destDir := filepath.Join(topDir, "dest2")

	// Ensure we are working with a schema2 image.
	// dir: accepts any manifest format, i.e. this makes …/input2 a schema2 source which cannot be asked to produce schema1 like ordinary docker: registries can.
	assertSkopeoSucceeds(c, "", "copy", "docker://busybox", "dir:"+input2Dir)
	verifyManifestMIMEType(c, input2Dir, manifest.DockerV2Schema2MediaType)
	// 2→2 (the "f2t2" in tag means "from 2 to 2")
	assertSkopeoSucceeds(c, "", "copy", "--dest-tls-verify=false", "dir:"+input2Dir, schema2Registry+":f2t2")
	assertSkopeoSucceeds(c, "", "copy", "--src-tls-verify=false", schema2Registry+":f2t2", "dir:"+destDir)
	verifyManifestMIMEType(c, destDir, manifest.DockerV2Schema2MediaType)
	// 2→1; we will use the result as a schema1 image for further tests.
	assertSkopeoSucceeds(c, "", "copy", "--dest-tls-verify=false", "dir:"+input2Dir, schema1Registry+":f2t1")
	assertSkopeoSucceeds(c, "", "copy", "--src-tls-verify=false", schema1Registry+":f2t1", "dir:"+input1Dir)
	verifyManifestMIMEType(c, input1Dir, manifest.DockerV2Schema1SignedMediaType)
	// 1→1
	assertSkopeoSucceeds(c, "", "copy", "--dest-tls-verify=false", "dir:"+input1Dir, schema1Registry+":f1t1")
	assertSkopeoSucceeds(c, "", "copy", "--src-tls-verify=false", schema1Registry+":f1t1", "dir:"+destDir)
	verifyManifestMIMEType(c, destDir, manifest.DockerV2Schema1SignedMediaType)
	// 1→2: image stays unmodified schema1
	assertSkopeoSucceeds(c, "", "copy", "--dest-tls-verify=false", "dir:"+input1Dir, schema2Registry+":f1t2")
	assertSkopeoSucceeds(c, "", "copy", "--src-tls-verify=false", schema2Registry+":f1t2", "dir:"+destDir)
	verifyManifestMIMEType(c, destDir, manifest.DockerV2Schema1SignedMediaType)
}

// Verify manifest in a dir: image at dir is expectedMIMEType.
func verifyManifestMIMEType(c *check.C, dir string, expectedMIMEType string) {
	manifestBlob, err := ioutil.ReadFile(filepath.Join(dir, "manifest.json"))
	c.Assert(err, check.IsNil)
	mimeType := manifest.GuessMIMEType(manifestBlob)
	c.Assert(mimeType, check.Equals, expectedMIMEType)
}
