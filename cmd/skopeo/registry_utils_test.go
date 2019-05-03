package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAuthTypeNone(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { testServer.Close() }()

	reg := NewRegistry(testServer.URL)
	reg.getAuthType()
	assert.Contains(t, reg.authType, noneAuth)
}

func TestGetAuthTypeToken(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("www-authenticate", "Bearer realm=\"www.example.com\",service=\"SUSE Linux Docker Registry\",scope=\"registry:catalog:*\",error=\"invalid_token\"")
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { testServer.Close() }()

	reg := NewRegistry(testServer.URL)
	reg.getAuthType()
	assert.Contains(t, reg.authType, tokenAuth)
}

func TestGetToken(t *testing.T) {

	// Test server to deliver token
	testServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"token":"abcdefg"}`))
	}))
	defer func() { testServer1.Close() }()

	// Second test server needed to deliver header containing URL of first test server
	testServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("www-authenticate", "Bearer realm=\""+testServer1.URL+"\",service=\"SUSE Linux Docker Registry\",scope=\"registry:catalog:*\",error=\"invalid_token\"")
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { testServer2.Close() }()

	token := getToken(testServer2.URL)
	assert.Equal(t, "abcdefg", token)
}

func TestGetRepoTags(t *testing.T) {

	repo := "opensuse/leap"
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		outMap := map[string][]string{
			"tags": {"15.0", "15.1", "latest"},
		}
		j, _ := json.Marshal(outMap)
		w.Write([]byte(j))
	}))
	defer func() { testServer.Close() }()

	reg := NewRegistry(testServer.URL)
	reg.getAuthType()
	repoTags := reg.getRepoTags(repo)
	assert.Contains(t, repoTags, "15.1")
}

func TestGetV2Catalog(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		outMap := map[string][]string{
			"repositories": {"opensuse/leap", "opensuse/tumbleweed", "not/a/real/repo"},
		}
		j, _ := json.Marshal(outMap)
		w.Write([]byte(j))
	}))
	defer func() { testServer.Close() }()

	reg := NewRegistry(testServer.URL)
	reg.authType = noneAuth
	reg.getV2Catalog()
	assert.Contains(t, reg.repos, "opensuse/tumbleweed")
}

func TestGetAllReposWithTags(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		outMap := map[string][]string{
			"repositories": {"opensuse/leap"},
			"tags":         {"15.0", "15.1", "latest"},
		}
		j, _ := json.Marshal(outMap)
		w.Write([]byte(j))
	}))
	defer func() { testServer.Close() }()

	reg := NewRegistry(testServer.URL)

	// Capture Stdout
	rescueStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	reg.getAllReposWithTags()

	w.Close()
	out, _ := ioutil.ReadAll(r)
	os.Stdout = rescueStdout

	matchedRepo, _ := regexp.MatchString("opensuse/leap", string(out))
	matchedTag, _ := regexp.MatchString("15.1", string(out))

	assert.True(t, matchedRepo)
	assert.True(t, matchedTag)
}
