package main

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/go-check/check"
)

const (
	privateRegistryURL0 = "127.0.0.1:5000"
	privateRegistryURL1 = "127.0.0.1:5001"
	privateRegistryURL2 = "127.0.0.1:5002"
	privateRegistryURL3 = "127.0.0.1:5003"
	privateRegistryURL4 = "127.0.0.1:5004"

	busyboxLatestDigest = "sha256:65ce39ce3eb0997074a460adfb568d0b9f0f6a4392d97b6035630c9d7bf92402"
)

var (
	localDockerBusybox = fmt.Sprintf("docker://%s/busybox@%s", privateRegistryURL1, busyboxLatestDigest)
)

func Test(t *testing.T) {
	check.TestingT(t)
}

func init() {
	check.Suite(&SkopeoSuite{})
}

type SkopeoSuite struct {
	regV1         *testRegistryV1
	regV2         *testRegistryV2
	regV2Shema1   *testRegistryV2
	regV1WithAuth *testRegistryV1 // does v1 support auth?
	regV2WithAuth *testRegistryV2
}

func (s *SkopeoSuite) SetUpSuite(c *check.C) {

}

func (s *SkopeoSuite) TearDownSuite(c *check.C) {

}

func (s *SkopeoSuite) SetUpTest(c *check.C) {
	_, err := exec.LookPath(skopeoBinary)
	c.Assert(err, check.IsNil)

	s.regV1 = setupRegistryV1At(c, privateRegistryURL0, false) // TODO:(runcom)
	s.regV2 = setupRegistryV2At(c, privateRegistryURL1, false, false)
	s.regV2Shema1 = setupRegistryV2At(c, privateRegistryURL2, false, true)
	s.regV1WithAuth = setupRegistryV1At(c, privateRegistryURL3, true) // not used
	s.regV2WithAuth = setupRegistryV2At(c, privateRegistryURL4, true, false)

	// TODO(upload with a tag as soon as we can instead of relying on an hardcoded digest)
	cmd := exec.Command(skopeoBinary, "copy", "dir:images/busybox-latest", "docker://"+s.regV2.url+"/busybox")
	c.Assert(cmd.Run(), check.IsNil)
}

func (s *SkopeoSuite) TearDownTest(c *check.C) {
	// not checking V1 registries now...
	if s.regV2 != nil {
		s.regV2.Close()
	}
	if s.regV2Shema1 != nil {
		s.regV2Shema1.Close()
	}
	if s.regV2WithAuth != nil {
		//cmd := exec.Command("docker", "logout", s.regV2WithAuth)
		//c.Assert(cmd.Run(), check.IsNil)
		s.regV2WithAuth.Close()
	}
}

// TODO like dockerCmd but much easier, just out,err
//func skopeoCmd()

func (s *SkopeoSuite) TestVersion(c *check.C) {
	wanted := fmt.Sprintf(".*%s version .*", skopeoBinary)
	assertSkopeoSucceeds(c, wanted, "--version")
}

const (
	errFetchManifestRegexp = ".*error fetching manifest: status code: %s.*"
)

func (s *SkopeoSuite) TestCanAuthToPrivateRegistryV2WithoutDockerCfg(c *check.C) {
	// TODO(runcom)
	c.Skip("we need to restore --username --password flags!")
	wanted := fmt.Sprintf(errFetchManifestRegexp, "401")
	assertSkopeoFails(c, wanted, "--docker-cfg=''", "--username="+s.regV2WithAuth.username, "--password="+s.regV2WithAuth.password, "inspect", fmt.Sprintf("docker://%s/busybox:latest", s.regV2WithAuth.url))
}

func (s *SkopeoSuite) TestNeedAuthToPrivateRegistryV2WithoutDockerCfg(c *check.C) {
	// TODO(runcom): mock the empty docker-cfg by removing it in the test itself (?)
	c.Skip("mock empty docker config")
	wanted := fmt.Sprintf(errFetchManifestRegexp, "401")
	assertSkopeoFails(c, wanted, "--docker-cfg=''", "inspect", fmt.Sprintf("docker://%s/busybox:latest", s.regV2WithAuth.url))
}

// TODO(runcom): as soon as we can push to registries ensure you can inspect here
// not just get image not found :)
func (s *SkopeoSuite) TestNoNeedAuthToPrivateRegistryV2ImageNotFound(c *check.C) {
	out, err := exec.Command(skopeoBinary, "inspect", fmt.Sprintf("docker://%s/busybox:latest", s.regV2.url)).CombinedOutput()
	c.Assert(err, check.NotNil, check.Commentf(string(out)))
	wanted := fmt.Sprintf(errFetchManifestRegexp, "404")
	c.Assert(string(out), check.Matches, "(?s)"+wanted) // (?s) : '.' will also match newlines
	wanted = fmt.Sprintf(errFetchManifestRegexp, "401")
	c.Assert(string(out), check.Not(check.Matches), "(?s)"+wanted) // (?s) : '.' will also match newlines
}
