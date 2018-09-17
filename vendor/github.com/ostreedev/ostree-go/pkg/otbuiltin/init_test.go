package otbuiltin

import (
	"os"
	"testing"
)

func TestInitSuccess(t *testing.T) {
	// Make a base directory in which all of our test data resides
	baseDir := "/tmp/otbuiltin-test/"
	err := os.Mkdir(baseDir, 0777)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	defer os.RemoveAll(baseDir)
	// Make a directory in which the repo should exist
	repoDir := baseDir + "repo"
	err = os.Mkdir(repoDir, 0777)
	if err != nil {
		t.Errorf("%s", err)
		return
	}

	// Initialize the repo
	inited, err := Init(repoDir, NewInitOptions())
	if err != nil {
		t.Errorf("%s", err)
		return
	} else if !inited {
		t.Errorf("Cannot test commit: failed to initialize repo")
		return
	}
}

func TestInitBareUser(t *testing.T) {
	// Make a base directory in which all of our test data resides
	baseDir := "/tmp/otbuiltin-test/"
	err := os.Mkdir(baseDir, 0777)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	defer os.RemoveAll(baseDir)
	// Make a directory in which the repo should exist
	repoDir := baseDir + "repo"
	err = os.Mkdir(repoDir, 0777)
	if err != nil {
		t.Errorf("%s", err)
		return
	}

	// Initialize the repo
	initOpts := NewInitOptions()
	initOpts.Mode = "bare-user"
	inited, err := Init(repoDir, initOpts)
	if err != nil {
		t.Errorf("%s", err)
		return
	} else if !inited {
		t.Errorf("Cannot test commit: failed to initialize repo")
		return
	}
}
