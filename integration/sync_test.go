package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/go-check/check"
)

const (
	// A repository with a path with multiple components in it which
	// contains multiple tags, preferably with some tags pointing to
	// manifest lists, and with some tags that don't.
	pullableRepo = "k8s.gcr.io/coredns/coredns"
	// A tagged image in the repository that we can inspect and copy.
	pullableTaggedImage = "k8s.gcr.io/coredns/coredns:v1.6.6"
	// A tagged manifest list in the repository that we can inspect and copy.
	pullableTaggedManifestList = "k8s.gcr.io/coredns/coredns:v1.8.0"
	// A repository containing multiple tags, some of which are for
	// manifest lists, and which includes a "latest" tag.  We specify the
	// name here without a tag.
	pullableRepoWithLatestTag = "k8s.gcr.io/pause"
)

func init() {
	check.Suite(&SyncSuite{})
}

type SyncSuite struct {
	cluster  *openshiftCluster
	registry *testRegistryV2
	gpgHome  string
}

func (s *SyncSuite) SetUpSuite(c *check.C) {
	const registryAuth = false
	const registrySchema1 = false

	if os.Getenv("SKOPEO_LOCAL_TESTS") == "1" {
		c.Log("Running tests without a container")
		fmt.Printf("NOTE: tests requires a V2 registry at url=%s, with auth=%t, schema1=%t \n", v2DockerRegistryURL, registryAuth, registrySchema1)
		return
	}

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
	s.registry = setupRegistryV2At(c, v2DockerRegistryURL, registryAuth, registrySchema1)

	gpgHome, err := ioutil.TempDir("", "skopeo-gpg")
	c.Assert(err, check.IsNil)
	s.gpgHome = gpgHome
	os.Setenv("GNUPGHOME", s.gpgHome)

	for _, key := range []string{"personal", "official"} {
		batchInput := fmt.Sprintf("Key-Type: RSA\nName-Real: Test key - %s\nName-email: %s@example.com\n%%no-protection\n%%commit\n",
			key, key)
		runCommandWithInput(c, batchInput, gpgBinary, "--batch", "--gen-key")

		out := combinedOutputOfCommand(c, gpgBinary, "--armor", "--export", fmt.Sprintf("%s@example.com", key))
		err := ioutil.WriteFile(filepath.Join(s.gpgHome, fmt.Sprintf("%s-pubkey.gpg", key)),
			[]byte(out), 0600)
		c.Assert(err, check.IsNil)
	}
}

func (s *SyncSuite) TearDownSuite(c *check.C) {
	if os.Getenv("SKOPEO_LOCAL_TESTS") == "1" {
		return
	}

	if s.gpgHome != "" {
		os.RemoveAll(s.gpgHome)
	}
	if s.registry != nil {
		s.registry.Close()
	}
	if s.cluster != nil {
		s.cluster.tearDown(c)
	}
}

func (s *SyncSuite) TestDocker2DirTagged(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)

	// FIXME: It would be nice to use one of the local Docker registries instead of needing an Internet connection.
	image := pullableTaggedImage
	imageRef, err := docker.ParseReference(fmt.Sprintf("//%s", image))
	c.Assert(err, check.IsNil)
	imagePath := imageRef.DockerReference().String()

	dir1 := path.Join(tmpDir, "dir1")
	dir2 := path.Join(tmpDir, "dir2")

	// sync docker => dir
	assertSkopeoSucceeds(c, "", "sync", "--scoped", "--src", "docker", "--dest", "dir", image, dir1)
	_, err = os.Stat(path.Join(dir1, imagePath, "manifest.json"))
	c.Assert(err, check.IsNil)

	// copy docker => dir
	assertSkopeoSucceeds(c, "", "copy", "docker://"+image, "dir:"+dir2)
	_, err = os.Stat(path.Join(dir2, "manifest.json"))
	c.Assert(err, check.IsNil)

	out := combinedOutputOfCommand(c, "diff", "-urN", path.Join(dir1, imagePath), dir2)
	c.Assert(out, check.Equals, "")
}

func (s *SyncSuite) TestDocker2DirTaggedAll(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)

	// FIXME: It would be nice to use one of the local Docker registries instead of needing an Internet connection.
	image := pullableTaggedManifestList
	imageRef, err := docker.ParseReference(fmt.Sprintf("//%s", image))
	c.Assert(err, check.IsNil)
	imagePath := imageRef.DockerReference().String()

	dir1 := path.Join(tmpDir, "dir1")
	dir2 := path.Join(tmpDir, "dir2")

	// sync docker => dir
	assertSkopeoSucceeds(c, "", "sync", "--all", "--scoped", "--src", "docker", "--dest", "dir", image, dir1)
	_, err = os.Stat(path.Join(dir1, imagePath, "manifest.json"))
	c.Assert(err, check.IsNil)

	// copy docker => dir
	assertSkopeoSucceeds(c, "", "copy", "--all", "docker://"+image, "dir:"+dir2)
	_, err = os.Stat(path.Join(dir2, "manifest.json"))
	c.Assert(err, check.IsNil)

	out := combinedOutputOfCommand(c, "diff", "-urN", path.Join(dir1, imagePath), dir2)
	c.Assert(out, check.Equals, "")
}

func (s *SyncSuite) TestScoped(c *check.C) {
	// FIXME: It would be nice to use one of the local Docker registries instead of needing an Internet connection.
	image := pullableTaggedImage
	imageRef, err := docker.ParseReference(fmt.Sprintf("//%s", image))
	c.Assert(err, check.IsNil)
	imagePath := imageRef.DockerReference().String()

	dir1, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	assertSkopeoSucceeds(c, "", "sync", "--src", "docker", "--dest", "dir", image, dir1)
	_, err = os.Stat(path.Join(dir1, path.Base(imagePath), "manifest.json"))
	c.Assert(err, check.IsNil)

	assertSkopeoSucceeds(c, "", "sync", "--scoped", "--src", "docker", "--dest", "dir", image, dir1)
	_, err = os.Stat(path.Join(dir1, imagePath, "manifest.json"))
	c.Assert(err, check.IsNil)

	os.RemoveAll(dir1)
}

func (s *SyncSuite) TestDirIsNotOverwritten(c *check.C) {
	// FIXME: It would be nice to use one of the local Docker registries instead of needing an Internet connection.
	image := pullableRepoWithLatestTag
	imageRef, err := docker.ParseReference(fmt.Sprintf("//%s", image))
	c.Assert(err, check.IsNil)
	imagePath := imageRef.DockerReference().String()

	// make a copy of the image in the local registry
	assertSkopeoSucceeds(c, "", "copy", "--dest-tls-verify=false", "docker://"+image, "docker://"+path.Join(v2DockerRegistryURL, reference.Path(imageRef.DockerReference())))

	//sync upstream image to dir, not scoped
	dir1, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	assertSkopeoSucceeds(c, "", "sync", "--src", "docker", "--dest", "dir", image, dir1)
	_, err = os.Stat(path.Join(dir1, path.Base(imagePath), "manifest.json"))
	c.Assert(err, check.IsNil)

	//sync local registry image to dir, not scoped
	assertSkopeoFails(c, ".*Refusing to overwrite destination directory.*", "sync", "--src-tls-verify=false", "--src", "docker", "--dest", "dir", path.Join(v2DockerRegistryURL, reference.Path(imageRef.DockerReference())), dir1)

	//sync local registry image to dir, scoped
	imageRef, err = docker.ParseReference(fmt.Sprintf("//%s", path.Join(v2DockerRegistryURL, reference.Path(imageRef.DockerReference()))))
	c.Assert(err, check.IsNil)
	imagePath = imageRef.DockerReference().String()
	assertSkopeoSucceeds(c, "", "sync", "--scoped", "--src-tls-verify=false", "--src", "docker", "--dest", "dir", path.Join(v2DockerRegistryURL, reference.Path(imageRef.DockerReference())), dir1)
	_, err = os.Stat(path.Join(dir1, imagePath, "manifest.json"))
	c.Assert(err, check.IsNil)
	os.RemoveAll(dir1)
}

func (s *SyncSuite) TestDocker2DirUntagged(c *check.C) {

	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)

	// FIXME: It would be nice to use one of the local Docker registries instead of needing an Internet connection.
	image := pullableRepo
	imageRef, err := docker.ParseReference(fmt.Sprintf("//%s", image))
	c.Assert(err, check.IsNil)
	imagePath := imageRef.DockerReference().String()

	dir1 := path.Join(tmpDir, "dir1")
	assertSkopeoSucceeds(c, "", "sync", "--scoped", "--src", "docker", "--dest", "dir", image, dir1)

	sysCtx := types.SystemContext{}
	tags, err := docker.GetRepositoryTags(context.Background(), &sysCtx, imageRef)
	c.Assert(err, check.IsNil)
	c.Check(len(tags), check.Not(check.Equals), 0)

	nManifests, err := filepath.Glob(path.Join(dir1, path.Dir(imagePath), "*", "manifest.json"))
	c.Assert(err, check.IsNil)
	c.Assert(len(nManifests), check.Equals, len(tags))
}

func (s *SyncSuite) TestYamlUntagged(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)
	dir1 := path.Join(tmpDir, "dir1")

	image := pullableRepo
	imageRef, err := docker.ParseReference(fmt.Sprintf("//%s", image))
	c.Assert(err, check.IsNil)
	imagePath := imageRef.DockerReference().Name()

	sysCtx := types.SystemContext{}
	tags, err := docker.GetRepositoryTags(context.Background(), &sysCtx, imageRef)
	c.Assert(err, check.IsNil)
	c.Check(len(tags), check.Not(check.Equals), 0)

	yamlConfig := fmt.Sprintf(`
%s:
  images:
    %s: []
`, reference.Domain(imageRef.DockerReference()), reference.Path(imageRef.DockerReference()))

	// sync to the local registry
	yamlFile := path.Join(tmpDir, "registries.yaml")
	ioutil.WriteFile(yamlFile, []byte(yamlConfig), 0644)
	assertSkopeoSucceeds(c, "", "sync", "--scoped", "--src", "yaml", "--dest", "docker", "--dest-tls-verify=false", yamlFile, v2DockerRegistryURL)
	// sync back from local registry to a folder
	os.Remove(yamlFile)
	yamlConfig = fmt.Sprintf(`
%s:
  tls-verify: false
  images:
    %s: []
`, v2DockerRegistryURL, imagePath)

	ioutil.WriteFile(yamlFile, []byte(yamlConfig), 0644)
	assertSkopeoSucceeds(c, "", "sync", "--scoped", "--src", "yaml", "--dest", "dir", yamlFile, dir1)

	sysCtx = types.SystemContext{
		DockerInsecureSkipTLSVerify: types.NewOptionalBool(true),
	}
	localImageRef, err := docker.ParseReference(fmt.Sprintf("//%s/%s", v2DockerRegistryURL, imagePath))
	c.Assert(err, check.IsNil)
	localTags, err := docker.GetRepositoryTags(context.Background(), &sysCtx, localImageRef)
	c.Assert(err, check.IsNil)
	c.Check(len(localTags), check.Not(check.Equals), 0)
	c.Assert(len(localTags), check.Equals, len(tags))

	nManifests := 0
	//count the number of manifest.json in dir1
	err = filepath.Walk(dir1, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "manifest.json" {
			nManifests++
			return filepath.SkipDir
		}
		return nil
	})
	c.Assert(err, check.IsNil)
	c.Assert(nManifests, check.Equals, len(tags))
}

func (s *SyncSuite) TestYamlRegex2Dir(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)
	dir1 := path.Join(tmpDir, "dir1")

	yamlConfig := `
k8s.gcr.io:
  images-by-tag-regex:
    pause: ^[12]\.0$  # regex string test
`
	// the       ↑    regex strings always matches only 2 images
	var nTags = 2
	c.Assert(nTags, check.Not(check.Equals), 0)

	yamlFile := path.Join(tmpDir, "registries.yaml")
	ioutil.WriteFile(yamlFile, []byte(yamlConfig), 0644)
	assertSkopeoSucceeds(c, "", "sync", "--scoped", "--src", "yaml", "--dest", "dir", yamlFile, dir1)

	nManifests := 0
	err = filepath.Walk(dir1, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "manifest.json" {
			nManifests++
			return filepath.SkipDir
		}
		return nil
	})
	c.Assert(err, check.IsNil)
	c.Assert(nManifests, check.Equals, nTags)
}

func (s *SyncSuite) TestYamlDigest2Dir(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)
	dir1 := path.Join(tmpDir, "dir1")

	yamlConfig := `
k8s.gcr.io:
  images:
    pause:
    - sha256:59eec8837a4d942cc19a52b8c09ea75121acc38114a2c68b98983ce9356b8610
`
	yamlFile := path.Join(tmpDir, "registries.yaml")
	ioutil.WriteFile(yamlFile, []byte(yamlConfig), 0644)
	assertSkopeoSucceeds(c, "", "sync", "--scoped", "--src", "yaml", "--dest", "dir", yamlFile, dir1)

	nManifests := 0
	err = filepath.Walk(dir1, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "manifest.json" {
			nManifests++
			return filepath.SkipDir
		}
		return nil
	})
	c.Assert(err, check.IsNil)
	c.Assert(nManifests, check.Equals, 1)
}

func (s *SyncSuite) TestYaml2Dir(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)
	dir1 := path.Join(tmpDir, "dir1")

	yamlConfig := `
k8s.gcr.io:
  images:
    coredns/coredns:
      - v1.8.0
      - v1.7.1
    k8s-dns-kube-dns:
      - 1.14.12
      - 1.14.13
    pause:
      - latest

quay.io:
  images:
    quay/busybox:
      - latest`

	// get the number of tags
	re := regexp.MustCompile(`^ +- +[^:/ ]+`)
	var nTags int
	for _, l := range strings.Split(yamlConfig, "\n") {
		if re.MatchString(l) {
			nTags++
		}
	}
	c.Assert(nTags, check.Not(check.Equals), 0)

	yamlFile := path.Join(tmpDir, "registries.yaml")
	ioutil.WriteFile(yamlFile, []byte(yamlConfig), 0644)
	assertSkopeoSucceeds(c, "", "sync", "--scoped", "--src", "yaml", "--dest", "dir", yamlFile, dir1)

	nManifests := 0
	err = filepath.Walk(dir1, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "manifest.json" {
			nManifests++
			return filepath.SkipDir
		}
		return nil
	})
	c.Assert(err, check.IsNil)
	c.Assert(nManifests, check.Equals, nTags)
}

func (s *SyncSuite) TestYamlTLSVerify(c *check.C) {
	const localRegURL = "docker://" + v2DockerRegistryURL + "/"
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)
	dir1 := path.Join(tmpDir, "dir1")
	image := pullableRepoWithLatestTag
	tag := "latest"

	// FIXME: It would be nice to use one of the local Docker registries instead of needing an Internet connection.
	// copy docker => docker
	assertSkopeoSucceeds(c, "", "copy", "--dest-tls-verify=false", "docker://"+image+":"+tag, localRegURL+image+":"+tag)

	yamlTemplate := `
%s:
  %s
  images:
    %s:
      - %s`

	testCfg := []struct {
		tlsVerify string
		msg       string
		checker   func(c *check.C, regexp string, args ...string)
	}{
		{
			tlsVerify: "tls-verify: false",
			msg:       "",
			checker:   assertSkopeoSucceeds,
		},
		{
			tlsVerify: "tls-verify: true",
			msg:       ".*server gave HTTP response to HTTPS client.*",
			checker:   assertSkopeoFails,
		},
		// no "tls-verify" line means default TLS verify must be ON
		{
			tlsVerify: "",
			msg:       ".*server gave HTTP response to HTTPS client.*",
			checker:   assertSkopeoFails,
		},
	}

	for _, cfg := range testCfg {
		yamlConfig := fmt.Sprintf(yamlTemplate, v2DockerRegistryURL, cfg.tlsVerify, image, tag)
		yamlFile := path.Join(tmpDir, "registries.yaml")
		ioutil.WriteFile(yamlFile, []byte(yamlConfig), 0644)

		cfg.checker(c, cfg.msg, "sync", "--scoped", "--src", "yaml", "--dest", "dir", yamlFile, dir1)
		os.Remove(yamlFile)
		os.RemoveAll(dir1)
	}

}

func (s *SyncSuite) TestDocker2DockerTagged(c *check.C) {
	const localRegURL = "docker://" + v2DockerRegistryURL + "/"

	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)

	// FIXME: It would be nice to use one of the local Docker registries instead of needing an Internet connection.
	image := pullableTaggedImage
	imageRef, err := docker.ParseReference(fmt.Sprintf("//%s", image))
	c.Assert(err, check.IsNil)
	imagePath := imageRef.DockerReference().String()

	dir1 := path.Join(tmpDir, "dir1")
	dir2 := path.Join(tmpDir, "dir2")

	// sync docker => docker
	assertSkopeoSucceeds(c, "", "sync", "--scoped", "--dest-tls-verify=false", "--src", "docker", "--dest", "docker", image, v2DockerRegistryURL)

	// copy docker => dir
	assertSkopeoSucceeds(c, "", "copy", "docker://"+image, "dir:"+dir1)
	_, err = os.Stat(path.Join(dir1, "manifest.json"))
	c.Assert(err, check.IsNil)

	// copy docker => dir
	assertSkopeoSucceeds(c, "", "copy", "--src-tls-verify=false", localRegURL+imagePath, "dir:"+dir2)
	_, err = os.Stat(path.Join(dir2, "manifest.json"))
	c.Assert(err, check.IsNil)

	out := combinedOutputOfCommand(c, "diff", "-urN", dir1, dir2)
	c.Assert(out, check.Equals, "")
}

func (s *SyncSuite) TestDir2DockerTagged(c *check.C) {
	const localRegURL = "docker://" + v2DockerRegistryURL + "/"

	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)

	// FIXME: It would be nice to use one of the local Docker registries instead of needing an Internet connection.
	image := pullableRepoWithLatestTag

	dir1 := path.Join(tmpDir, "dir1")
	err = os.Mkdir(dir1, 0755)
	c.Assert(err, check.IsNil)
	dir2 := path.Join(tmpDir, "dir2")
	err = os.Mkdir(dir2, 0755)
	c.Assert(err, check.IsNil)

	// create leading dirs
	err = os.MkdirAll(path.Dir(path.Join(dir1, image)), 0755)
	c.Assert(err, check.IsNil)

	// copy docker => dir
	assertSkopeoSucceeds(c, "", "copy", "docker://"+image, "dir:"+path.Join(dir1, image))
	_, err = os.Stat(path.Join(dir1, image, "manifest.json"))
	c.Assert(err, check.IsNil)

	// sync dir => docker
	assertSkopeoSucceeds(c, "", "sync", "--scoped", "--dest-tls-verify=false", "--src", "dir", "--dest", "docker", dir1, v2DockerRegistryURL)

	// create leading dirs
	err = os.MkdirAll(path.Dir(path.Join(dir2, image)), 0755)
	c.Assert(err, check.IsNil)

	// copy docker => dir
	assertSkopeoSucceeds(c, "", "copy", "--src-tls-verify=false", localRegURL+image, "dir:"+path.Join(dir2, image))
	_, err = os.Stat(path.Join(dir2, image, "manifest.json"))
	c.Assert(err, check.IsNil)

	out := combinedOutputOfCommand(c, "diff", "-urN", dir1, dir2)
	c.Assert(out, check.Equals, "")
}

func (s *SyncSuite) TestFailsWithDir2Dir(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)

	dir1 := path.Join(tmpDir, "dir1")
	dir2 := path.Join(tmpDir, "dir2")

	// sync dir => dir is not allowed
	assertSkopeoFails(c, ".*sync from 'dir' to 'dir' not implemented.*", "sync", "--scoped", "--src", "dir", "--dest", "dir", dir1, dir2)
}

func (s *SyncSuite) TestFailsNoSourceImages(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)

	assertSkopeoFails(c, ".*No images to sync found in .*",
		"sync", "--scoped", "--dest-tls-verify=false", "--src", "dir", "--dest", "docker", tmpDir, v2DockerRegistryURL)

	assertSkopeoFails(c, ".*No images to sync found in .*",
		"sync", "--scoped", "--dest-tls-verify=false", "--src", "docker", "--dest", "docker", "hopefully_no_images_will_ever_be_called_like_this", v2DockerRegistryURL)
}

func (s *SyncSuite) TestFailsWithDockerSourceNoRegistry(c *check.C) {
	const regURL = "google.com/namespace/imagename"

	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)

	//untagged
	assertSkopeoFails(c, ".*invalid status code from registry 404.*",
		"sync", "--scoped", "--src", "docker", "--dest", "dir", regURL, tmpDir)

	//tagged
	assertSkopeoFails(c, ".*invalid status code from registry 404.*",
		"sync", "--scoped", "--src", "docker", "--dest", "dir", regURL+":thetag", tmpDir)
}

func (s *SyncSuite) TestFailsWithDockerSourceUnauthorized(c *check.C) {
	const repo = "privateimagenamethatshouldnotbepublic"
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)

	//untagged
	assertSkopeoFails(c, ".*Registry disallows tag list retrieval.*",
		"sync", "--scoped", "--src", "docker", "--dest", "dir", repo, tmpDir)

	//tagged
	assertSkopeoFails(c, ".*unauthorized: authentication required.*",
		"sync", "--scoped", "--src", "docker", "--dest", "dir", repo+":thetag", tmpDir)
}

func (s *SyncSuite) TestFailsWithDockerSourceNotExisting(c *check.C) {
	repo := path.Join(v2DockerRegistryURL, "imagedoesnotexist")
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpDir)

	//untagged
	assertSkopeoFails(c, ".*invalid status code from registry 404.*",
		"sync", "--scoped", "--src-tls-verify=false", "--src", "docker", "--dest", "dir", repo, tmpDir)

	//tagged
	assertSkopeoFails(c, ".*Error reading manifest.*",
		"sync", "--scoped", "--src-tls-verify=false", "--src", "docker", "--dest", "dir", repo+":thetag", tmpDir)
}

func (s *SyncSuite) TestFailsWithDirSourceNotExisting(c *check.C) {
	// Make sure the dir does not exist!
	tmpDir, err := ioutil.TempDir("", "skopeo-sync-test")
	c.Assert(err, check.IsNil)
	err = os.RemoveAll(tmpDir)
	c.Assert(err, check.IsNil)
	_, err = os.Stat(path.Join(tmpDir))
	c.Check(os.IsNotExist(err), check.Equals, true)

	assertSkopeoFails(c, ".*no such file or directory.*",
		"sync", "--scoped", "--dest-tls-verify=false", "--src", "dir", "--dest", "docker", tmpDir, v2DockerRegistryURL)
}
