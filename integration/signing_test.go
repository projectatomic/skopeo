package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/containers/image/v5/signature"
	"github.com/go-check/check"
)

const (
	gpgBinary = "gpg"
)

func init() {
	check.Suite(&SigningSuite{})
}

type SigningSuite struct {
	gpgHome     string
	fingerprint string
}

func findFingerprint(lineBytes []byte) (string, error) {
	lines := string(lineBytes)
	for _, line := range strings.Split(lines, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 10 && fields[0] == "fpr" {
			return fields[9], nil
		}
	}
	return "", errors.New("No fingerprint found")
}

func (s *SigningSuite) SetUpSuite(c *check.C) {
	_, err := exec.LookPath(skopeoBinary)
	c.Assert(err, check.IsNil)

	s.gpgHome, err = ioutil.TempDir("", "skopeo-gpg")
	c.Assert(err, check.IsNil)
	os.Setenv("GNUPGHOME", s.gpgHome)

	runCommandWithInput(c, "Key-Type: RSA\nName-Real: Testing user\n%no-protection\n%commit\n", gpgBinary, "--homedir", s.gpgHome, "--batch", "--gen-key")

	lines, err := exec.Command(gpgBinary, "--homedir", s.gpgHome, "--with-colons", "--no-permission-warning", "--fingerprint").Output()
	c.Assert(err, check.IsNil)
	s.fingerprint, err = findFingerprint(lines)
	c.Assert(err, check.IsNil)
}

func (s *SigningSuite) TearDownSuite(c *check.C) {
	if s.gpgHome != "" {
		err := os.RemoveAll(s.gpgHome)
		c.Assert(err, check.IsNil)
	}
	s.gpgHome = ""

	os.Unsetenv("GNUPGHOME")
}

func (s *SigningSuite) TestSignVerifySmoke(c *check.C) {
	mech, _, err := signature.NewEphemeralGPGSigningMechanism([]byte{})
	c.Assert(err, check.IsNil)
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil { // FIXME? Test that verification and policy enforcement works, using signatures from fixtures
		c.Skip(fmt.Sprintf("Signing not supported: %v", err))
	}

	manifestPath := "fixtures/image.manifest.json"
	dockerReference := "testing/smoketest"

	sigOutput, err := ioutil.TempFile("", "sig")
	c.Assert(err, check.IsNil)
	defer os.Remove(sigOutput.Name())
	assertSkopeoSucceeds(c, "^$", "standalone-sign", "-o", sigOutput.Name(),
		manifestPath, dockerReference, s.fingerprint)

	expected := fmt.Sprintf("^Signature verified, digest %s\n$", TestImageManifestDigest)
	assertSkopeoSucceeds(c, expected, "standalone-verify", manifestPath,
		dockerReference, s.fingerprint, sigOutput.Name())
}
