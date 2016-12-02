package main

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/go-check/check"
	"github.com/projectatomic/skopeo/version"
)

const (
	privateRegistryURL0 = "127.0.0.1:5000"
	privateRegistryURL1 = "127.0.0.1:5001"
	privateRegistryURL2 = "127.0.0.1:5002"
	privateRegistryURL3 = "127.0.0.1:5003"
	privateRegistryURL4 = "127.0.0.1:5004"
)

func Test(t *testing.T) {
	check.TestingT(t)
}

func init() {
	check.Suite(&SkopeoSuite{})
}

type SkopeoSuite struct {
	regV2         *testRegistryV2
	regV2Shema1   *testRegistryV2
	regV2WithAuth *testRegistryV2
}

func (s *SkopeoSuite) SetUpSuite(c *check.C) {

}

func (s *SkopeoSuite) TearDownSuite(c *check.C) {

}

func (s *SkopeoSuite) SetUpTest(c *check.C) {
	_, err := exec.LookPath(skopeoBinary)
	c.Assert(err, check.IsNil)

	s.regV2 = setupRegistryV2At(c, privateRegistryURL1, false, false)
	s.regV2Shema1 = setupRegistryV2At(c, privateRegistryURL2, false, true)
	s.regV2WithAuth = setupRegistryV2At(c, privateRegistryURL4, true, false)
}

func (s *SkopeoSuite) TearDownTest(c *check.C) {
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
	wanted := fmt.Sprintf(".*%s version %s.*", skopeoBinary, version.Version)
	assertSkopeoSucceeds(c, wanted, "--version")
}

func (s *SkopeoSuite) TestCanAuthToPrivateRegistryV2WithoutDockerCfg(c *check.C) {
	wanted := ".*manifest unknown: manifest unknown.*"
	assertSkopeoFails(c, wanted, "--tls-verify=false", "inspect", "--creds="+s.regV2WithAuth.username+":"+s.regV2WithAuth.password, fmt.Sprintf("docker://%s/busybox:latest", s.regV2WithAuth.url))
}

func (s *SkopeoSuite) TestNeedAuthToPrivateRegistryV2WithoutDockerCfg(c *check.C) {
	wanted := ".*unauthorized: authentication required.*"
	assertSkopeoFails(c, wanted, "--tls-verify=false", "inspect", fmt.Sprintf("docker://%s/busybox:latest", s.regV2WithAuth.url))
}

// TODO(runcom): as soon as we can push to registries ensure you can inspect here
// not just get image not found :)
func (s *SkopeoSuite) TestNoNeedAuthToPrivateRegistryV2ImageNotFound(c *check.C) {
	out, err := exec.Command(skopeoBinary, "--tls-verify=false", "inspect", fmt.Sprintf("docker://%s/busybox:latest", s.regV2.url)).CombinedOutput()
	c.Assert(err, check.NotNil, check.Commentf(string(out)))
	wanted := ".*manifest unknown.*"
	c.Assert(string(out), check.Matches, "(?s)"+wanted) // (?s) : '.' will also match newlines
	wanted = ".*unauthorized: authentication required.*"
	c.Assert(string(out), check.Not(check.Matches), "(?s)"+wanted) // (?s) : '.' will also match newlines
}
