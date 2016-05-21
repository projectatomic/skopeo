package main

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

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

// ConsumeAndLogOutput takes (f, err) from an exec.*Pipe(), and causes all output to it to be logged to c.
func ConsumeAndLogOutput(c *check.C, id string, f io.ReadCloser, err error) {
	c.Assert(err, check.IsNil)
	go func() {
		defer func() {
			f.Close()
			c.Logf("Output %s: Closed", id)
		}()
		buf := make([]byte, 0, 1024)
		for {
			c.Logf("Output %s: waiting", id)
			n, err := f.Read(buf)
			c.Logf("Output %s: got %d,%#v: %#v", id, n, err, buf[:n])
			if n <= 0 {
				break
			}
		}
	}()
}

func (s *SigningSuite) SetUpTest(c *check.C) {
	_, err := exec.LookPath(skopeoBinary)
	c.Assert(err, check.IsNil)

	s.gpgHome, err = ioutil.TempDir("", "skopeo-gpg")
	c.Assert(err, check.IsNil)
	os.Setenv("GNUPGHOME", s.gpgHome)

	cmd := exec.Command(gpgBinary, "--homedir", s.gpgHome, "--batch", "--gen-key")
	stdin, err := cmd.StdinPipe()
	c.Assert(err, check.IsNil)
	stdout, err := cmd.StdoutPipe()
	ConsumeAndLogOutput(c, "gen-key stdout", stdout, err)
	stderr, err := cmd.StderrPipe()
	ConsumeAndLogOutput(c, "gen-key stderr", stderr, err)
	err = cmd.Start()
	c.Assert(err, check.IsNil)
	_, err = stdin.Write([]byte("Key-Type: RSA\nName-Real: Testing user\n%commit\n"))
	c.Assert(err, check.IsNil)
	err = stdin.Close()
	c.Assert(err, check.IsNil)
	err = cmd.Wait()
	c.Assert(err, check.IsNil)

	lines, err := exec.Command(gpgBinary, "--homedir", s.gpgHome, "--with-colons", "--no-permission-warning", "--fingerprint").Output()
	c.Assert(err, check.IsNil)
	s.fingerprint, err = findFingerprint(lines)
	c.Assert(err, check.IsNil)
}

func (s *SigningSuite) TearDownTest(c *check.C) {
	if s.gpgHome != "" {
		err := os.RemoveAll(s.gpgHome)
		c.Assert(err, check.IsNil)
	}
	s.gpgHome = ""

	os.Unsetenv("GNUPGHOME")
}

func (s *SigningSuite) TestSignVerifySmoke(c *check.C) {
	manifestPath := "fixtures/image.manifest.json"
	dockerReference := "testing/smoketest"

	sigOutput, err := ioutil.TempFile("", "sig")
	c.Assert(err, check.IsNil)
	defer os.Remove(sigOutput.Name())
	out, err := exec.Command(skopeoBinary, "standalone-sign", "-o", sigOutput.Name(),
		manifestPath, dockerReference, s.fingerprint).CombinedOutput()
	c.Assert(err, check.IsNil, check.Commentf("%s", out))
	c.Assert(string(out), check.Equals, "")

	out, err = exec.Command(skopeoBinary, "standalone-verify", manifestPath,
		dockerReference, s.fingerprint, sigOutput.Name()).CombinedOutput()
	c.Assert(err, check.IsNil, check.Commentf("%s", out))
	c.Assert(string(out), check.Equals, "Signature verified, digest "+TestImageManifestDigest+"\n")
}
