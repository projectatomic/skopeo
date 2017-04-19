package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/go-check/check"
)

const skopeoBinary = "skopeo"

// consumeAndLogOutputStream takes (f, err) from an exec.*Pipe(), and causes all output to it to be logged to c.
func consumeAndLogOutputStream(c *check.C, id string, f io.ReadCloser, err error) {
	c.Assert(err, check.IsNil)
	go func() {
		defer func() {
			f.Close()
			c.Logf("Output %s: Closed", id)
		}()
		buf := make([]byte, 1024)
		for {
			c.Logf("Output %s: waiting", id)
			n, err := f.Read(buf)
			c.Logf("Output %s: got %d,%#v: %s", id, n, err, strings.TrimSuffix(string(buf[:n]), "\n"))
			if n <= 0 {
				break
			}
		}
	}()
}

// consumeAndLogOutputs causes all output to stdout and stderr from an *exec.Cmd to be logged to c
func consumeAndLogOutputs(c *check.C, id string, cmd *exec.Cmd) {
	stdout, err := cmd.StdoutPipe()
	consumeAndLogOutputStream(c, id+" stdout", stdout, err)
	stderr, err := cmd.StderrPipe()
	consumeAndLogOutputStream(c, id+" stderr", stderr, err)
}

// combinedOutputOfCommand runs a command as if exec.Command().CombinedOutput(), verifies that the exit status is 0, and returns the output,
// or terminates c on failure.
func combinedOutputOfCommand(c *check.C, name string, args ...string) string {
	c.Logf("Running %s %s", name, strings.Join(args, " "))
	out, err := exec.Command(name, args...).CombinedOutput()
	c.Assert(err, check.IsNil, check.Commentf("%s", out))
	return string(out)
}

// assertSkopeoSucceeds runs a skopeo command as if exec.Command().CombinedOutput, verifies that the exit status is 0,
// and optionally that the output matches a multi-line regexp if it is nonempty;
// or terminates c on failure
func assertSkopeoSucceeds(c *check.C, regexp string, args ...string) {
	c.Logf("Running %s %s", skopeoBinary, strings.Join(args, " "))
	out, err := exec.Command(skopeoBinary, args...).CombinedOutput()
	c.Assert(err, check.IsNil, check.Commentf("%s", out))
	if regexp != "" {
		c.Assert(string(out), check.Matches, "(?s)"+regexp) // (?s) : '.' will also match newlines
	}
}

// assertSkopeoFails runs a skopeo command as if exec.Command().CombinedOutput, verifies that the exit status is 0,
// and that the output matches a multi-line regexp;
// or terminates c on failure
func assertSkopeoFails(c *check.C, regexp string, args ...string) {
	c.Logf("Running %s %s", skopeoBinary, strings.Join(args, " "))
	out, err := exec.Command(skopeoBinary, args...).CombinedOutput()
	c.Assert(err, check.NotNil, check.Commentf("%s", out))
	c.Assert(string(out), check.Matches, "(?s)"+regexp) // (?s) : '.' will also match newlines
}

// runCommandWithInput runs a command as if exec.Command(), sending it the input to stdin,
// and verifies that the exit status is 0, or terminates c on failure.
func runCommandWithInput(c *check.C, input string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	runExecCmdWithInput(c, cmd, input)
}

// runExecCmdWithInput runs an exec.Cmd, sending it the input to stdin,
// and verifies that the exit status is 0, or terminates c on failure.
func runExecCmdWithInput(c *check.C, cmd *exec.Cmd, input string) {
	c.Logf("Running %s %s", cmd.Path, strings.Join(cmd.Args, " "))
	consumeAndLogOutputs(c, cmd.Path+" "+strings.Join(cmd.Args, " "), cmd)
	stdin, err := cmd.StdinPipe()
	c.Assert(err, check.IsNil)
	err = cmd.Start()
	c.Assert(err, check.IsNil)
	_, err = stdin.Write([]byte(input))
	c.Assert(err, check.IsNil)
	err = stdin.Close()
	c.Assert(err, check.IsNil)
	err = cmd.Wait()
	c.Assert(err, check.IsNil)
}

// isPortOpen returns true iff the specified port on localhost is open.
func isPortOpen(port int) bool {
	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// newPortChecker sets up a portOpen channel which will receive true after the specified port is open.
// The checking can be aborted by sending a value to the terminate channel, which the caller should
// always do using
// defer func() {terminate <- true}()
func newPortChecker(c *check.C, port int) (portOpen <-chan bool, terminate chan<- bool) {
	portOpenBidi := make(chan bool)
	// Buffered, so that sending a terminate request after the goroutine has exited does not block.
	terminateBidi := make(chan bool, 1)

	go func() {
		defer func() {
			c.Logf("Port checker for port %d exiting", port)
		}()
		for {
			c.Logf("Checking for port %d...", port)
			if isPortOpen(port) {
				c.Logf("Port %d open", port)
				portOpenBidi <- true
				return
			}
			c.Logf("Sleeping for port %d", port)
			sleepChan := time.After(100 * time.Millisecond)
			select {
			case <-sleepChan: // Try again
				c.Logf("Sleeping for port %d done, will retry", port)
			case <-terminateBidi:
				c.Logf("Check for port %d terminated", port)
				return
			}
		}
	}()
	return portOpenBidi, terminateBidi
}

// modifyEnviron modifies os.Environ()-like list of name=value assignments to set name to value.
func modifyEnviron(env []string, name, value string) []string {
	prefix := name + "="
	res := []string{}
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			res = append(res, e)
		}
	}
	return append(res, prefix+value)
}

// fileFromFixtureFixture applies edits to inputPath and returns a path to the temporary file.
// Callers should defer os.Remove(the_returned_path)
func fileFromFixture(c *check.C, inputPath string, edits map[string]string) string {
	contents, err := ioutil.ReadFile(inputPath)
	c.Assert(err, check.IsNil)
	for template, value := range edits {
		updated := bytes.Replace(contents, []byte(template), []byte(value), -1)
		c.Assert(bytes.Equal(updated, contents), check.Equals, false, check.Commentf("Replacing %s in %#v failed", template, string(contents))) // Verify that the template has matched something and we are not silently ignoring it.
		contents = updated
	}

	file, err := ioutil.TempFile("", "policy.json")
	c.Assert(err, check.IsNil)
	path := file.Name()

	_, err = file.Write(contents)
	c.Assert(err, check.IsNil)
	err = file.Close()
	c.Assert(err, check.IsNil)
	return path
}
