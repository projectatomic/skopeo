package otbuiltin

import (
	"fmt"
	"os"
	"testing"

	"github.com/14rcole/gopopulate"
)

func TestCheckoutSuccessProcessOneBranch(t *testing.T) {
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

	checkoutOpts := NewCheckoutOptions()
	checkoutDir := baseDir + "checkout"

	// Don't create the checkout dir, it will be created in the checkout function
	err = Checkout(repoDir, checkoutDir, branch, checkoutOpts)
	defer os.RemoveAll(branch)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
}

func TestCheckoutSuccessProcessOneCommit(t *testing.T) {

}

func TestCheckoutFailProcessOne(t *testing.T) {

}

func TestCheckoutSuccessProcessMany(t *testing.T) {

}

func TestCheckoutFailProcessMany(t *testing.T) {

}
