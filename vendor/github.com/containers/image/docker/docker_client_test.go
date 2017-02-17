package docker

import (
	"encoding/base64"
	"encoding/json"
	//"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/containers/image/types"
	"github.com/containers/storage/pkg/homedir"
)

func TestGetAuth(t *testing.T) {
	origHomeDir := homedir.Get()
	tmpDir, err := ioutil.TempDir("", "test_docker_client_get_auth")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("using temporary home directory: %q", tmpDir)
	// override homedir
	os.Setenv(homedir.Key(), tmpDir)
	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			t.Logf("failed to cleanup temporary home directory %q: %v", tmpDir, err)
		}
		os.Setenv(homedir.Key(), origHomeDir)
	}()

	configDir := filepath.Join(tmpDir, ".docker")
	if err := os.Mkdir(configDir, 0750); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.json")

	for _, tc := range []struct {
		name             string
		hostname         string
		authConfig       testAuthConfig
		expectedUsername string
		expectedPassword string
		expectedError    error
		ctx              *types.SystemContext
	}{
		{
			name:       "empty hostname",
			authConfig: makeTestAuthConfig(testAuthConfigDataMap{"localhost:5000": testAuthConfigData{"bob", "password"}}),
		},
		{
			name:     "no auth config",
			hostname: "index.docker.io",
		},
		{
			name:             "match one",
			hostname:         "example.org",
			authConfig:       makeTestAuthConfig(testAuthConfigDataMap{"example.org": testAuthConfigData{"joe", "mypass"}}),
			expectedUsername: "joe",
			expectedPassword: "mypass",
		},
		{
			name:       "match none",
			hostname:   "registry.example.org",
			authConfig: makeTestAuthConfig(testAuthConfigDataMap{"example.org": testAuthConfigData{"joe", "mypass"}}),
		},
		{
			name:     "match docker.io",
			hostname: "docker.io",
			authConfig: makeTestAuthConfig(testAuthConfigDataMap{
				"example.org":     testAuthConfigData{"example", "org"},
				"index.docker.io": testAuthConfigData{"index", "docker.io"},
				"docker.io":       testAuthConfigData{"docker", "io"},
			}),
			expectedUsername: "docker",
			expectedPassword: "io",
		},
		{
			name:     "match docker.io normalized",
			hostname: "docker.io",
			authConfig: makeTestAuthConfig(testAuthConfigDataMap{
				"example.org":                testAuthConfigData{"bob", "pw"},
				"https://index.docker.io/v1": testAuthConfigData{"alice", "wp"},
			}),
			expectedUsername: "alice",
			expectedPassword: "wp",
		},
		{
			name:     "normalize registry",
			hostname: "https://docker.io/v1",
			authConfig: makeTestAuthConfig(testAuthConfigDataMap{
				"docker.io":      testAuthConfigData{"user", "pw"},
				"localhost:5000": testAuthConfigData{"joe", "pass"},
			}),
			expectedUsername: "user",
			expectedPassword: "pw",
		},
		{
			name:     "match localhost",
			hostname: "http://localhost",
			authConfig: makeTestAuthConfig(testAuthConfigDataMap{
				"docker.io":   testAuthConfigData{"user", "pw"},
				"localhost":   testAuthConfigData{"joe", "pass"},
				"example.com": testAuthConfigData{"alice", "pwd"},
			}),
			expectedUsername: "joe",
			expectedPassword: "pass",
		},
		{
			name:     "match ip",
			hostname: "10.10.3.56:5000",
			authConfig: makeTestAuthConfig(testAuthConfigDataMap{
				"10.10.30.45":     testAuthConfigData{"user", "pw"},
				"localhost":       testAuthConfigData{"joe", "pass"},
				"10.10.3.56":      testAuthConfigData{"alice", "pwd"},
				"10.10.3.56:5000": testAuthConfigData{"me", "mine"},
			}),
			expectedUsername: "me",
			expectedPassword: "mine",
		},
		{
			name:     "match port",
			hostname: "https://localhost:5000",
			authConfig: makeTestAuthConfig(testAuthConfigDataMap{
				"https://127.0.0.1:5000": testAuthConfigData{"user", "pw"},
				"http://localhost":       testAuthConfigData{"joe", "pass"},
				"https://localhost:5001": testAuthConfigData{"alice", "pwd"},
				"localhost:5000":         testAuthConfigData{"me", "mine"},
			}),
			expectedUsername: "me",
			expectedPassword: "mine",
		},
		{
			name:     "use system context",
			hostname: "example.org",
			authConfig: makeTestAuthConfig(testAuthConfigDataMap{
				"example.org": testAuthConfigData{"user", "pw"},
			}),
			expectedUsername: "foo",
			expectedPassword: "bar",
			ctx: &types.SystemContext{
				DockerAuthConfig: &types.DockerAuthConfig{
					Username: "foo",
					Password: "bar",
				},
			},
		},
	} {
		contents, err := json.MarshalIndent(&tc.authConfig, "", "  ")
		if err != nil {
			t.Errorf("[%s] failed to marshal authConfig: %v", tc.name, err)
			continue
		}
		if err := ioutil.WriteFile(configPath, contents, 0640); err != nil {
			t.Errorf("[%s] failed to write file %q: %v", tc.name, configPath, err)
			continue
		}

		var ctx *types.SystemContext
		if tc.ctx != nil {
			ctx = tc.ctx
		}
		username, password, err := getAuth(ctx, tc.hostname)
		if err == nil && tc.expectedError != nil {
			t.Errorf("[%s] got unexpected non error and username=%q, password=%q", tc.name, username, password)
			continue
		}
		if err != nil && tc.expectedError == nil {
			t.Errorf("[%s] got unexpected error: %#+v", tc.name, err)
			continue
		}
		if !reflect.DeepEqual(err, tc.expectedError) {
			t.Errorf("[%s] got unexpected error: %#+v != %#+v", tc.name, err, tc.expectedError)
			continue
		}

		if username != tc.expectedUsername {
			t.Errorf("[%s] got unexpected user name: %q != %q", tc.name, username, tc.expectedUsername)
		}
		if password != tc.expectedPassword {
			t.Errorf("[%s] got unexpected user name: %q != %q", tc.name, password, tc.expectedPassword)
		}
	}
}

func TestGetAuthFromLegacyFile(t *testing.T) {
	origHomeDir := homedir.Get()
	tmpDir, err := ioutil.TempDir("", "test_docker_client_get_auth")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("using temporary home directory: %q", tmpDir)
	// override homedir
	os.Setenv(homedir.Key(), tmpDir)
	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			t.Logf("failed to cleanup temporary home directory %q: %v", tmpDir, err)
		}
		os.Setenv(homedir.Key(), origHomeDir)
	}()

	configPath := filepath.Join(tmpDir, ".dockercfg")

	for _, tc := range []struct {
		name             string
		hostname         string
		authConfig       testAuthConfig
		expectedUsername string
		expectedPassword string
		expectedError    error
	}{
		{
			name:     "normalize registry",
			hostname: "https://docker.io/v1",
			authConfig: makeTestAuthConfig(testAuthConfigDataMap{
				"docker.io":      testAuthConfigData{"user", "pw"},
				"localhost:5000": testAuthConfigData{"joe", "pass"},
			}),
			expectedUsername: "user",
			expectedPassword: "pw",
		},
		{
			name:     "ignore schema and path",
			hostname: "http://index.docker.io/v1",
			authConfig: makeTestAuthConfig(testAuthConfigDataMap{
				"docker.io/v2":         testAuthConfigData{"user", "pw"},
				"https://localhost/v1": testAuthConfigData{"joe", "pwd"},
			}),
			expectedUsername: "user",
			expectedPassword: "pw",
		},
	} {
		contents, err := json.MarshalIndent(&tc.authConfig.Auths, "", "  ")
		if err != nil {
			t.Errorf("[%s] failed to marshal authConfig: %v", tc.name, err)
			continue
		}
		if err := ioutil.WriteFile(configPath, contents, 0640); err != nil {
			t.Errorf("[%s] failed to write file %q: %v", tc.name, configPath, err)
			continue
		}

		username, password, err := getAuth(nil, tc.hostname)
		if err == nil && tc.expectedError != nil {
			t.Errorf("[%s] got unexpected non error and username=%q, password=%q", tc.name, username, password)
			continue
		}
		if err != nil && tc.expectedError == nil {
			t.Errorf("[%s] got unexpected error: %#+v", tc.name, err)
			continue
		}
		if !reflect.DeepEqual(err, tc.expectedError) {
			t.Errorf("[%s] got unexpected error: %#+v != %#+v", tc.name, err, tc.expectedError)
			continue
		}

		if username != tc.expectedUsername {
			t.Errorf("[%s] got unexpected user name: %q != %q", tc.name, username, tc.expectedUsername)
		}
		if password != tc.expectedPassword {
			t.Errorf("[%s] got unexpected user name: %q != %q", tc.name, password, tc.expectedPassword)
		}
	}
}

func TestGetAuthPreferNewConfig(t *testing.T) {
	origHomeDir := homedir.Get()
	tmpDir, err := ioutil.TempDir("", "test_docker_client_get_auth")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("using temporary home directory: %q", tmpDir)
	// override homedir
	os.Setenv(homedir.Key(), tmpDir)
	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			t.Logf("failed to cleanup temporary home directory %q: %v", tmpDir, err)
		}
		os.Setenv(homedir.Key(), origHomeDir)
	}()

	configDir := filepath.Join(tmpDir, ".docker")
	if err := os.Mkdir(configDir, 0750); err != nil {
		t.Fatal(err)
	}

	for _, data := range []struct {
		path string
		ac   interface{}
	}{
		{
			filepath.Join(configDir, "config.json"),
			makeTestAuthConfig(testAuthConfigDataMap{
				"https://index.docker.io/v1/": testAuthConfigData{"alice", "pass"},
			}),
		},
		{
			filepath.Join(tmpDir, ".dockercfg"),
			makeTestAuthConfig(testAuthConfigDataMap{
				"https://index.docker.io/v1/": testAuthConfigData{"bob", "pw"},
			}).Auths,
		},
	} {
		contents, err := json.MarshalIndent(&data.ac, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal authConfig: %v", err)
		}
		if err := ioutil.WriteFile(data.path, contents, 0640); err != nil {
			t.Fatalf("failed to write file %q: %v", data.path, err)
		}
	}

	username, password, err := getAuth(nil, "index.docker.io")
	if err != nil {
		t.Fatalf("got unexpected error: %#+v", err)
	}

	if username != "alice" {
		t.Fatalf("got unexpected user name: %q != %q", username, "alice")
	}
	if password != "pass" {
		t.Fatalf("got unexpected user name: %q != %q", password, "pass")
	}
}

func TestGetAuthFailsOnBadInput(t *testing.T) {
	origHomeDir := homedir.Get()
	tmpDir, err := ioutil.TempDir("", "test_docker_client_get_auth")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("using temporary home directory: %q", tmpDir)
	// override homedir
	os.Setenv(homedir.Key(), tmpDir)
	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			t.Logf("failed to cleanup temporary home directory %q: %v", tmpDir, err)
		}
		os.Setenv(homedir.Key(), origHomeDir)
	}()

	configDir := filepath.Join(tmpDir, ".docker")
	if err := os.Mkdir(configDir, 0750); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.json")

	// no config file present
	username, password, err := getAuth(nil, "index.docker.io")
	if err != nil {
		t.Fatalf("got unexpected error: %#+v", err)
	}
	if len(username) > 0 || len(password) > 0 {
		t.Fatalf("got unexpected not empty username/password: %q/%q", username, password)
	}

	if err := ioutil.WriteFile(configPath, []byte("Json rocks! Unless it doesn't."), 0640); err != nil {
		t.Fatalf("failed to write file %q: %v", configPath, err)
	}
	username, password, err = getAuth(nil, "index.docker.io")
	if err == nil {
		t.Fatalf("got unexpected non-error: username=%q, password=%q", username, password)
	}
	if _, ok := err.(*json.SyntaxError); !ok {
		t.Fatalf("expected os.PathError, not: %#+v", err)
	}

	// remove the invalid config file
	os.RemoveAll(configPath)
	// no config file present
	username, password, err = getAuth(nil, "index.docker.io")
	if err != nil {
		t.Fatalf("got unexpected error: %#+v", err)
	}
	if len(username) > 0 || len(password) > 0 {
		t.Fatalf("got unexpected not empty username/password: %q/%q", username, password)
	}

	configPath = filepath.Join(tmpDir, ".dockercfg")
	if err := ioutil.WriteFile(configPath, []byte("I'm certainly not a json string."), 0640); err != nil {
		t.Fatalf("failed to write file %q: %v", configPath, err)
	}
	username, password, err = getAuth(nil, "index.docker.io")
	if err == nil {
		t.Fatalf("got unexpected non-error: username=%q, password=%q", username, password)
	}
	if _, ok := err.(*json.SyntaxError); !ok {
		t.Fatalf("expected os.PathError, not: %#+v", err)
	}
}

type testAuthConfigData struct {
	username string
	password string
}

type testAuthConfigDataMap map[string]testAuthConfigData

type testAuthConfigEntry struct {
	Auth string `json:"auth,omitempty"`
}

type testAuthConfig struct {
	Auths map[string]testAuthConfigEntry `json:"auths"`
}

// encodeAuth creates an auth value from given authConfig data to be stored in auth config file.
// Inspired by github.com/docker/docker/cliconfig/config.go v1.10.3.
func encodeAuth(authConfig *testAuthConfigData) string {
	authStr := authConfig.username + ":" + authConfig.password
	msg := []byte(authStr)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(msg)))
	base64.StdEncoding.Encode(encoded, msg)
	return string(encoded)
}

func makeTestAuthConfig(authConfigData map[string]testAuthConfigData) testAuthConfig {
	ac := testAuthConfig{
		Auths: make(map[string]testAuthConfigEntry),
	}
	for host, data := range authConfigData {
		ac.Auths[host] = testAuthConfigEntry{
			Auth: encodeAuth(&data),
		}
	}
	return ac
}
