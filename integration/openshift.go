package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/homedir"
	"github.com/go-check/check"
)

// openshiftCluster is an OpenShift API master and integrated registry
// running on localhost.
type openshiftCluster struct {
	c          *check.C
	workingDir string
	master     *exec.Cmd
	registry   *exec.Cmd
}

// startOpenshiftCluster creates a new openshiftCluster.
// WARNING: This affects state in users' home directory! Only run
// in isolated test environment.
func startOpenshiftCluster(c *check.C) *openshiftCluster {
	cluster := &openshiftCluster{c: c}

	dir, err := ioutil.TempDir("", "openshift-cluster")
	cluster.c.Assert(err, check.IsNil)
	cluster.workingDir = dir

	cluster.startMaster()
	cluster.startRegistry()
	cluster.ocLoginToProject()
	cluster.dockerLogin()
	cluster.relaxImageSignerPermissions()

	return cluster
}

// startMaster starts the OpenShift master (etcd+API server) and waits for it to be ready, or terminates on failure.
func (c *openshiftCluster) startMaster() {
	c.master = exec.Command("openshift", "start", "master")
	c.master.Dir = c.workingDir
	stdout, err := c.master.StdoutPipe()
	// Send both to the same pipe. This might cause the two streams to be mixed up,
	// but logging actually goes only to stderr - this primarily ensure we log any
	// unexpected output to stdout.
	c.master.Stderr = c.master.Stdout
	err = c.master.Start()
	c.c.Assert(err, check.IsNil)

	portOpen, terminatePortCheck := newPortChecker(c.c, 8443)
	defer func() {
		c.c.Logf("Terminating port check")
		terminatePortCheck <- true
	}()

	terminateLogCheck := make(chan bool, 1)
	logCheckFound := make(chan bool)
	go func() {
		defer func() {
			c.c.Logf("Log checker exiting")
		}()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			c.c.Logf("Log line: %s", line)
			if strings.Contains(line, "Started Origin Controllers") {
				logCheckFound <- true
				return
				// FIXME? We stop reading from stdout; could this block the master?
			}
			// Note: we can block before we get here.
			select {
			case <-terminateLogCheck:
				c.c.Logf("terminated")
				return
			default:
				// Do not block here and read the next line.
			}
		}
		logCheckFound <- false
	}()
	defer func() {
		c.c.Logf("Terminating log check")
		terminateLogCheck <- true
	}()

	gotPortCheck := false
	gotLogCheck := false
	for !gotPortCheck || !gotLogCheck {
		c.c.Logf("Waiting for master")
		select {
		case <-portOpen:
			c.c.Logf("port check done")
			gotPortCheck = true
		case found := <-logCheckFound:
			c.c.Logf("log check done, found: %t", found)
			if !found {
				c.c.Fatal("log check done, success message not found")
			}
			gotLogCheck = true
		}
	}
	c.c.Logf("OK, master started!")
}

// startRegistry starts the OpenShift registry and waits for it to be ready, or terminates on failure.
func (c *openshiftCluster) startRegistry() {
	//KUBECONFIG=openshift.local.config/master/openshift-registry.kubeconfig DOCKER_REGISTRY_URL=127.0.0.1:5000
	c.registry = exec.Command("dockerregistry", "/atomic-registry-config.yml")
	c.registry.Dir = c.workingDir
	c.registry.Env = os.Environ()
	c.registry.Env = modifyEnviron(c.registry.Env, "KUBECONFIG", "openshift.local.config/master/openshift-registry.kubeconfig")
	c.registry.Env = modifyEnviron(c.registry.Env, "DOCKER_REGISTRY_URL", "127.0.0.1:5000")
	consumeAndLogOutputs(c.c, "registry", c.registry)
	err := c.registry.Start()
	c.c.Assert(err, check.IsNil)

	portOpen, terminatePortCheck := newPortChecker(c.c, 5000)
	defer func() {
		terminatePortCheck <- true
	}()
	c.c.Logf("Waiting for registry to start")
	<-portOpen
	c.c.Logf("OK, Registry port open")
}

// ocLogin runs (oc login) and (oc new-project) on the cluster, or terminates on failure.
func (c *openshiftCluster) ocLoginToProject() {
	c.c.Logf("oc login")
	cmd := exec.Command("oc", "login", "--certificate-authority=openshift.local.config/master/ca.crt", "-u", "myuser", "-p", "mypw", "https://localhost:8443")
	cmd.Dir = c.workingDir
	out, err := cmd.CombinedOutput()
	c.c.Assert(err, check.IsNil, check.Commentf("%s", out))
	c.c.Assert(string(out), check.Matches, "(?s).*Login successful.*") // (?s) : '.' will also match newlines

	outString := combinedOutputOfCommand(c.c, "oc", "new-project", "myns")
	c.c.Assert(outString, check.Matches, `(?s).*Now using project "myns".*`) // (?s) : '.' will also match newlines
}

// dockerLogin simulates (docker login) to the cluster, or terminates on failure.
// We do not run (docker login) directly, because that requires a running daemon and a docker package.
func (c *openshiftCluster) dockerLogin() {
	dockerDir := filepath.Join(homedir.Get(), ".docker")
	err := os.Mkdir(dockerDir, 0700)
	c.c.Assert(err, check.IsNil)

	out := combinedOutputOfCommand(c.c, "oc", "config", "view", "-o", "json", "-o", "jsonpath={.users[*].user.token}")
	c.c.Logf("oc config value: %s", out)
	configJSON := fmt.Sprintf(`{
		"auths": {
			"localhost:5000": {
				"auth": "%s",
				"email": "unused"
			}
		}
	}`, base64.StdEncoding.EncodeToString([]byte("unused:"+out)))
	err = ioutil.WriteFile(filepath.Join(dockerDir, "config.json"), []byte(configJSON), 0600)
	c.c.Assert(err, check.IsNil)
}

// relaxImageSignerPermissions opens up the system:image-signer permissions so that
// anyone can work with signatures
// FIXME: This also allows anyone to DoS anyone else; this design is really not all
// that workable, but it is the best we can do for now.
func (c *openshiftCluster) relaxImageSignerPermissions() {
	cmd := exec.Command("oadm", "policy", "add-cluster-role-to-group", "system:image-signer", "system:authenticated")
	cmd.Dir = c.workingDir
	cmd.Env = os.Environ()
	cmd.Env = modifyEnviron(cmd.Env, "KUBECONFIG", "openshift.local.config/master/admin.kubeconfig")
	out, err := cmd.CombinedOutput()
	c.c.Assert(err, check.IsNil, check.Commentf("%s", string(out)))
	c.c.Assert(string(out), check.Equals, "")
}

// tearDown stops the cluster services and deletes (only some!) of the state.
func (c *openshiftCluster) tearDown() {
	if c.registry != nil && c.registry.Process != nil {
		c.registry.Process.Kill()
	}
	if c.master != nil && c.master.Process != nil {
		c.master.Process.Kill()
	}
	if c.workingDir != "" {
		os.RemoveAll(c.workingDir)
	}
}
