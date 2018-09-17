package otbuiltin

import (
	"fmt"
	"os"
	"testing"

	"github.com/14rcole/gopopulate"
)

func TestLogSuccess(t *testing.T) {
	// Make a base directory in which all of our test data resides
	baseDir := "/tmp/otbuiltin-test/"
	err := os.Mkdir(baseDir, 0777)
	if err != nil {
		t.Errorf("%s", err)
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
	if !inited || err != nil {
		fmt.Println("Cannot test commit: failed to initialize repo")
		return
	}

	//Make a new directory full of random data to commit
	commitDir := baseDir + "commit1"
	err = os.Mkdir(commitDir, 0777)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	err = gopopulate.PopulateDir(commitDir, "rd", 4, 4)
	if err != nil {
		t.Errorf("%s", err)
		return
	}

	//Test commit
	opts := NewCommitOptions()
	branch := "test-branch"
	ret, err := Commit(repoDir, commitDir, branch, opts)
	if err != nil {
		t.Errorf("%s", err)
	} else {
		fmt.Println(ret)
	}

	// Add more files to the commit dir and return an updated
	err = gopopulate.PopulateDir(commitDir, "rd", 4, 4)
	if err != nil {
		t.Errorf("%s", err)
		return
	}

	// Get the logs for the branch
	logOpts := NewLogOptions()
	entries, err := Log(repoDir, branch, logOpts)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	fmt.Printf("%+v\n", entries)
}
