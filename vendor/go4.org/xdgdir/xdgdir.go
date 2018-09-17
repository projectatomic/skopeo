/*
Copyright 2017 The go4 Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package xdgdir implements the Free Desktop Base Directory
// specification for locating directories.
//
// The specification is at
// http://standards.freedesktop.org/basedir-spec/basedir-spec-latest.html
package xdgdir // import "go4.org/xdgdir"

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
)

// Directories defined by the specification.
var (
	Data    Dir
	Config  Dir
	Cache   Dir
	Runtime Dir
)

func init() {
	// Placed in init for the sake of readable docs.
	Data = Dir{
		env:          "XDG_DATA_HOME",
		dirsEnv:      "XDG_DATA_DIRS",
		fallback:     ".local/share",
		dirsFallback: []string{"/usr/local/share", "/usr/share"},
	}
	Config = Dir{
		env:          "XDG_CONFIG_HOME",
		dirsEnv:      "XDG_CONFIG_DIRS",
		fallback:     ".config",
		dirsFallback: []string{"/etc/xdg"},
	}
	Cache = Dir{
		env:      "XDG_CACHE_HOME",
		fallback: ".cache",
	}
	Runtime = Dir{
		env:       "XDG_RUNTIME_DIR",
		userOwned: true,
	}
}

// A Dir is a logical base directory along with additional search
// directories.
type Dir struct {
	// env is the name of the environment variable for the base directory
	// relative to which files should be written.
	env string

	// dirsEnv is the name of the environment variable containing
	// preference-ordered base directories to search for files.
	dirsEnv string

	// fallback is the home-relative path to use if the variable named by
	// env is not set.
	fallback string

	// dirsFallback is the list of paths to use if the variable named by
	// dirsEnv is not set.
	dirsFallback []string

	// If userOwned is true, then for the directory to be considered
	// valid, it must be owned by the user with the mode 700.  This is
	// only used for XDG_RUNTIME_DIR.
	userOwned bool
}

// String returns the name of the primary environment variable for the
// directory.
func (d Dir) String() string {
	if d.env == "" {
		panic("xdgdir.Dir.String() on zero Dir")
	}
	return d.env
}

// Path returns the absolute path of the primary directory, or an empty
// string if there's no suitable directory present.  This is the path
// that should be used for writing files.
func (d Dir) Path() string {
	if d.env == "" {
		panic("xdgdir.Dir.Path() on zero Dir")
	}
	p := d.path()
	if p != "" && d.userOwned {
		info, err := os.Stat(p)
		if err != nil {
			return ""
		}
		if !info.IsDir() || info.Mode().Perm() != 0700 {
			return ""
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || int(st.Uid) != geteuid() {
			return ""
		}
	}
	return p
}

func (d Dir) path() string {
	if e := getenv(d.env); isValidPath(e) {
		return e
	}
	if d.fallback == "" {
		return ""
	}
	home := findHome()
	if home == "" {
		return ""
	}
	p := filepath.Join(home, d.fallback)
	if !isValidPath(p) {
		return ""
	}
	return p
}

// SearchPaths returns the list of paths (in descending order of
// preference) to search for files.
func (d Dir) SearchPaths() []string {
	if d.env == "" {
		panic("xdgdir.Dir.SearchPaths() on zero Dir")
	}
	var paths []string
	if p := d.Path(); p != "" {
		paths = append(paths, p)
	}
	if d.dirsEnv == "" {
		return paths
	}
	e := getenv(d.dirsEnv)
	if e == "" {
		paths = append(paths, d.dirsFallback...)
		return paths
	}
	epaths := filepath.SplitList(e)
	n := 0
	for _, p := range epaths {
		if isValidPath(p) {
			epaths[n] = p
			n++
		}
	}
	paths = append(paths, epaths[:n]...)
	return paths
}

// Open opens the named file inside the directory for reading.  If the
// directory has multiple search paths, each path is checked in order
// for the file and the first one found is opened.
func (d Dir) Open(name string) (*os.File, error) {
	if d.env == "" {
		return nil, errors.New("xdgdir: Open on zero Dir")
	}
	paths := d.SearchPaths()
	if len(paths) == 0 {
		return nil, fmt.Errorf("xdgdir: open %s: %s is invalid or not set", name, d.env)
	}
	var firstErr error
	for _, p := range paths {
		f, err := os.Open(filepath.Join(p, name))
		if err == nil {
			return f, nil
		} else if !os.IsNotExist(err) {
			firstErr = err
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, &os.PathError{
		Op:   "Open",
		Path: filepath.Join("$"+d.env, name),
		Err:  os.ErrNotExist,
	}
}

// Create creates the named file inside the directory mode 0666 (before
// umask), truncating it if it already exists.  Parent directories of
// the file will be created with mode 0700.
func (d Dir) Create(name string) (*os.File, error) {
	if d.env == "" {
		return nil, errors.New("xdgdir: Create on zero Dir")
	}
	p := d.Path()
	if p == "" {
		return nil, fmt.Errorf("xdgdir: create %s: %s is invalid or not set", name, d.env)
	}
	fp := filepath.Join(p, name)
	if err := os.MkdirAll(filepath.Dir(fp), 0700); err != nil {
		return nil, err
	}
	return os.Create(fp)
}

func isValidPath(path string) bool {
	return path != "" && filepath.IsAbs(path)
}

// findHome returns the user's home directory or the empty string if it
// can't be found.  It can be faked for testing.
var findHome = func() string {
	if h := getenv("HOME"); h != "" {
		return h
	}
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.HomeDir
}

// getenv retrieves an environment variable.  It can be faked for testing.
var getenv = os.Getenv

// geteuid retrieves the effective user ID of the process.  It can be faked for testing.
var geteuid = os.Geteuid
