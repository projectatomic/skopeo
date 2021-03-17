package main

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/containers/skopeo/version"
	"github.com/go-check/check"
)

const (
	privateRegistryURL0 = "127.0.0.1:5000"
	privateRegistryURL1 = "127.0.0.1:5001"
)

func Test(t *testing.T) {
	check.TestingT(t)
}

func init() {
	check.Suite(&SkopeoSuite{})
}

type SkopeoSuite struct {
	regV2         *testRegistryV2
	regV2WithAuth *testRegistryV2
}

func (s *SkopeoSuite) SetUpSuite(c *check.C) {
	_, err := exec.LookPath(skopeoBinary)
	c.Assert(err, check.IsNil)
	s.regV2 = setupRegistryV2At(c, privateRegistryURL0, false, false)
	s.regV2WithAuth = setupRegistryV2At(c, privateRegistryURL1, true, false)
}

func (s *SkopeoSuite) TearDownSuite(c *check.C) {
	if s.regV2 != nil {
		s.regV2.Close()
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

func (s *SkopeoSuite) TestCertDirInsteadOfCertPath(c *check.C) {
	wanted := ".*unknown flag: --cert-path.*"
	assertSkopeoFails(c, wanted, "--tls-verify=false", "inspect", fmt.Sprintf("docker://%s/busybox:latest", s.regV2WithAuth.url), "--cert-path=/")
	wanted = ".*unauthorized: authentication required.*"
	assertSkopeoFails(c, wanted, "--tls-verify=false", "inspect", fmt.Sprintf("docker://%s/busybox:latest", s.regV2WithAuth.url), "--cert-dir=/etc/docker/certs.d/")
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

func (s *SkopeoSuite) TestInspectFailsWhenReferenceIsInvalid(c *check.C) {
	assertSkopeoFails(c, `.*Invalid image name.*`, "inspect", "unknown")
}

func (s *SkopeoSuite) TestLoginLogout(c *check.C) {
	wanted := "^Login Succeeded!\n$"
	assertSkopeoSucceeds(c, wanted, "login", "--tls-verify=false", "--username="+s.regV2WithAuth.username, "--password="+s.regV2WithAuth.password, s.regV2WithAuth.url)
	// test --get-login returns username
	wanted = fmt.Sprintf("^%s\n$", s.regV2WithAuth.username)
	assertSkopeoSucceeds(c, wanted, "login", "--tls-verify=false", "--get-login", s.regV2WithAuth.url)
	// test logout
	wanted = fmt.Sprintf("^Removed login credentials for %s\n$", s.regV2WithAuth.url)
	assertSkopeoSucceeds(c, wanted, "logout", s.regV2WithAuth.url)
}

func (s *SkopeoSuite) TestCopyWithLocalAuth(c *check.C) {
	wanted := "^Login Succeeded!\n$"
	assertSkopeoSucceeds(c, wanted, "login", "--tls-verify=false", "--username="+s.regV2WithAuth.username, "--password="+s.regV2WithAuth.password, s.regV2WithAuth.url)
	// copy to private registry using local authentication
	imageName := fmt.Sprintf("docker://%s/busybox:mine", s.regV2WithAuth.url)
	assertSkopeoSucceeds(c, "", "copy", "--dest-tls-verify=false", "docker://docker.io/library/busybox:latest", imageName)
	// inspect from private registry
	assertSkopeoSucceeds(c, "", "inspect", "--tls-verify=false", imageName)
	// logout from the registry
	wanted = fmt.Sprintf("^Removed login credentials for %s\n$", s.regV2WithAuth.url)
	assertSkopeoSucceeds(c, wanted, "logout", s.regV2WithAuth.url)
	// inspect from private registry should fail after logout
	wanted = ".*unauthorized: authentication required.*"
	assertSkopeoFails(c, wanted, "inspect", "--tls-verify=false", imageName)
}
