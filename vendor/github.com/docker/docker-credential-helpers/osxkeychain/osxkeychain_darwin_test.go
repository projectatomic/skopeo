package osxkeychain

import (
	"errors"
	"fmt"
	"github.com/docker/docker-credential-helpers/credentials"
	"testing"
)

func TestOSXKeychainHelper(t *testing.T) {
	creds := &credentials.Credentials{
		ServerURL: "https://foobar.docker.io:2376/v1",
		Username:  "foobar",
		Secret:    "foobarbaz",
	}
	creds1 := &credentials.Credentials{
		ServerURL: "https://foobar.docker.io:2376/v2",
		Username:  "foobarbaz",
		Secret:    "foobar",
	}
	helper := Osxkeychain{}
	if err := helper.Add(creds); err != nil {
		t.Fatal(err)
	}

	username, secret, err := helper.Get(creds.ServerURL)
	if err != nil {
		t.Fatal(err)
	}

	if username != "foobar" {
		t.Fatalf("expected %s, got %s\n", "foobar", username)
	}

	if secret != "foobarbaz" {
		t.Fatalf("expected %s, got %s\n", "foobarbaz", secret)
	}

	auths, err := helper.List()
	if err != nil || len(auths) == 0 {
		t.Fatal(err)
	}

	helper.Add(creds1)
	defer helper.Delete(creds1.ServerURL)
	newauths, err := helper.List()
	if len(newauths)-len(auths) != 1 {
		if err == nil {
			t.Fatalf("Error: len(newauths): %d, len(auths): %d", len(newauths), len(auths))
		}
		t.Fatalf("Error: len(newauths): %d, len(auths): %d\n Error= %v", len(newauths), len(auths), err)
	}

	if err := helper.Delete(creds.ServerURL); err != nil {
		t.Fatal(err)
	}
}

// TestOSXKeychainHelperParseURL verifies that a // "scheme" is added to URLs,
// and that invalid URLs produce an error.
func TestOSXKeychainHelperParseURL(t *testing.T) {
	tests := []struct {
		url         string
		expectedURL string
		err         error
	}{
		{url: "foobar.docker.io", expectedURL: "//foobar.docker.io"},
		{url: "foobar.docker.io:2376", expectedURL: "//foobar.docker.io:2376"},
		{url: "//foobar.docker.io:2376", expectedURL: "//foobar.docker.io:2376"},
		{url: "http://foobar.docker.io:2376", expectedURL: "http://foobar.docker.io:2376"},
		{url: "https://foobar.docker.io:2376", expectedURL: "https://foobar.docker.io:2376"},
		{url: "https://foobar.docker.io:2376/some/path", expectedURL: "https://foobar.docker.io:2376/some/path"},
		{url: "https://foobar.docker.io:2376/some/other/path?foo=bar", expectedURL: "https://foobar.docker.io:2376/some/other/path"},
		{url: "/foobar.docker.io", err: errors.New("no hostname in URL")},
		{url: "ftp://foobar.docker.io:2376", err: errors.New("unsupported scheme: ftp")},
	}

	for _, te := range tests {
		u, err := parseURL(te.url)

		if te.err == nil && err != nil {
			t.Errorf("Error: failed to parse URL %q: %s", te.url, err)
			continue
		}
		if te.err != nil && err == nil {
			t.Errorf("Error: expected error %q, got none when parsing URL %q", te.err, te.url)
			continue
		}
		if te.err != nil && err.Error() != te.err.Error() {
			t.Errorf("Error: expected error %q, got %q when parsing URL %q", te.err, err, te.url)
			continue
		}
		if u != nil && u.String() != te.expectedURL {
			t.Errorf("Error: expected URL: %q, but got %q for URL: %q", te.expectedURL, u.String(), te.url)
		}
	}
}

// TestOSXKeychainHelperRetrieveAliases verifies that secrets can be accessed
// through variations on the URL
func TestOSXKeychainHelperRetrieveAliases(t *testing.T) {
	tests := []struct {
		storeURL string
		readURL  string
	}{
		// stored with port, retrieved without
		{"https://foobar.docker.io:2376", "https://foobar.docker.io"},

		// stored as https, retrieved without scheme
		{"https://foobar.docker.io:2376", "foobar.docker.io"},

		// stored with path, retrieved without
		{"https://foobar.docker.io:1234/one/two", "https://foobar.docker.io:1234"},
	}

	helper := Osxkeychain{}
	defer func() {
		for _, te := range tests {
			helper.Delete(te.storeURL)
		}
	}()

	// Clean store before testing.
	for _, te := range tests {
		helper.Delete(te.storeURL)
	}

	for _, te := range tests {
		c := &credentials.Credentials{ServerURL: te.storeURL, Username: "hello", Secret: "world"}
		if err := helper.Add(c); err != nil {
			t.Errorf("Error: failed to store secret for URL %q: %s", te.storeURL, err)
			continue
		}
		if _, _, err := helper.Get(te.readURL); err != nil {
			t.Errorf("Error: failed to read secret for URL %q using %q", te.storeURL, te.readURL)
		}
		helper.Delete(te.storeURL)
	}
}

// TestOSXKeychainHelperRetrieveStrict verifies that only matching secrets are
// returned.
func TestOSXKeychainHelperRetrieveStrict(t *testing.T) {
	tests := []struct {
		storeURL string
		readURL  string
	}{
		// stored as https, retrieved using http
		{"https://foobar.docker.io:2376", "http://foobar.docker.io:2376"},

		// stored as http, retrieved using https
		{"http://foobar.docker.io:2376", "https://foobar.docker.io:2376"},

		// same: stored as http, retrieved without a scheme specified (hence, using the default https://)
		{"http://foobar.docker.io", "foobar.docker.io:5678"},

		// non-matching ports
		{"https://foobar.docker.io:1234", "https://foobar.docker.io:5678"},

		// non-matching ports TODO is this desired behavior? The other way round does work
		//{"https://foobar.docker.io", "https://foobar.docker.io:5678"},

		// non-matching paths
		{"https://foobar.docker.io:1234/one/two", "https://foobar.docker.io:1234/five/six"},
	}

	helper := Osxkeychain{}
	defer func() {
		for _, te := range tests {
			helper.Delete(te.storeURL)
		}
	}()

	// Clean store before testing.
	for _, te := range tests {
		helper.Delete(te.storeURL)
	}

	for _, te := range tests {
		c := &credentials.Credentials{ServerURL: te.storeURL, Username: "hello", Secret: "world"}
		if err := helper.Add(c); err != nil {
			t.Errorf("Error: failed to store secret for URL %q: %s", te.storeURL, err)
			continue
		}
		if _, _, err := helper.Get(te.readURL); err == nil {
			t.Errorf("Error: managed to read secret for URL %q using %q, but should not be able to", te.storeURL, te.readURL)
		}
		helper.Delete(te.storeURL)
	}
}

// TestOSXKeychainHelperStoreRetrieve verifies that secrets stored in the
// the keychain can be read back using the URL that was used to store them.
func TestOSXKeychainHelperStoreRetrieve(t *testing.T) {
	tests := []struct {
		url string
	}{
		{url: "foobar.docker.io"},
		{url: "foobar.docker.io:2376"},
		{url: "//foobar.docker.io:2376"},
		{url: "https://foobar.docker.io:2376"},
		{url: "http://foobar.docker.io:2376"},
		{url: "https://foobar.docker.io:2376/some/path"},
		{url: "https://foobar.docker.io:2376/some/other/path"},
		{url: "https://foobar.docker.io:2376/some/other/path?foo=bar"},
	}

	helper := Osxkeychain{}
	defer func() {
		for _, te := range tests {
			helper.Delete(te.url)
		}
	}()

	// Clean store before testing.
	for _, te := range tests {
		helper.Delete(te.url)
	}

	// Note that we don't delete between individual tests here, to verify that
	// subsequent stores/overwrites don't affect storing / retrieving secrets.
	for i, te := range tests {
		c := &credentials.Credentials{
			ServerURL: te.url,
			Username:  fmt.Sprintf("user-%d", i),
			Secret:    fmt.Sprintf("secret-%d", i),
		}

		if err := helper.Add(c); err != nil {
			t.Errorf("Error: failed to store secret for URL: %s: %s", te.url, err)
			continue
		}
		user, secret, err := helper.Get(te.url)
		if err != nil {
			t.Errorf("Error: failed to read secret for URL %q: %s", te.url, err)
			continue
		}
		if user != c.Username {
			t.Errorf("Error: expected username %s, got username %s for URL: %s", c.Username, user, te.url)
		}
		if secret != c.Secret {
			t.Errorf("Error: expected secret %s, got secret %s for URL: %s", c.Secret, secret, te.url)
		}
	}
}

func TestMissingCredentials(t *testing.T) {
	helper := Osxkeychain{}
	_, _, err := helper.Get("https://adsfasdf.wrewerwer.com/asdfsdddd")
	if !credentials.IsErrCredentialsNotFound(err) {
		t.Fatalf("expected ErrCredentialsNotFound, got %v", err)
	}
}
