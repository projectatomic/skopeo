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

package xdgdir

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestDir_Path(t *testing.T) {
	td := newTempDir(t)
	defer td.cleanup()
	allopenDir := td.mkdir("allopen", 0777)
	readonlyDir := td.mkdir("readonly", 0400)
	secureDir := td.mkdir("secure", 0700)

	tests := []struct {
		dir     Dir
		env     env
		path    string
		geteuid func() int
	}{
		{
			dir:  Data,
			env:  env{"HOME": "/xHOMEx/me", "XDG_DATA_HOME": "/foo/data"},
			path: "/foo/data",
		},
		{
			dir:  Data,
			env:  env{"HOME": "/xHOMEx/me"},
			path: "/xHOMEx/me/.local/share",
		},
		{
			dir:  Data,
			env:  env{"HOME": "/xHOMEx/me", "XDG_DATA_HOME": "relative/path"},
			path: "/xHOMEx/me/.local/share",
		},
		{
			dir:  Data,
			env:  env{},
			path: "",
		},
		{
			dir:  Data,
			env:  env{"HOME": "relative/path"},
			path: "",
		},
		{
			dir:  Config,
			env:  env{"HOME": "/xHOMEx/me", "XDG_CONFIG_HOME": "/foo/config"},
			path: "/foo/config",
		},
		{
			dir:  Config,
			env:  env{"HOME": "/xHOMEx/me"},
			path: "/xHOMEx/me/.config",
		},
		{
			dir:  Config,
			env:  env{"HOME": "/xHOMEx/me", "XDG_CONFIG_HOME": "relative/path"},
			path: "/xHOMEx/me/.config",
		},
		{
			dir:  Config,
			env:  env{},
			path: "",
		},
		{
			dir:  Cache,
			env:  env{"HOME": "/xHOMEx/me", "XDG_CACHE_HOME": "/foo/cache"},
			path: "/foo/cache",
		},
		{
			dir:  Cache,
			env:  env{"HOME": "/xHOMEx/me"},
			path: "/xHOMEx/me/.cache",
		},
		{
			dir:  Cache,
			env:  env{"HOME": "/xHOMEx/me", "XDG_CACHE_HOME": "relative/path"},
			path: "/xHOMEx/me/.cache",
		},
		{
			dir:  Cache,
			env:  env{},
			path: "",
		},
		{
			dir:  Runtime,
			env:  env{"XDG_RUNTIME_DIR": secureDir},
			path: secureDir,
		},
		{
			dir:     Runtime,
			env:     env{"XDG_RUNTIME_DIR": secureDir},
			geteuid: func() int { return os.Geteuid() + 1 },
			path:    "",
		},
		{
			dir:  Runtime,
			env:  env{"XDG_RUNTIME_DIR": readonlyDir},
			path: "",
		},
		{
			dir:  Runtime,
			env:  env{"XDG_RUNTIME_DIR": allopenDir},
			path: "",
		},
		{
			dir:  Runtime,
			env:  env{"HOME": secureDir},
			path: "",
		},
		{
			dir:  Runtime,
			env:  env{},
			path: "",
		},
	}
	for _, test := range tests {
		test.env.set()
		if test.geteuid != nil {
			geteuid = test.geteuid
		} else {
			geteuid = os.Geteuid
		}
		if path := test.dir.Path(); path != test.path {
			var euidMod string
			if test.geteuid != nil {
				euidMod = " (euid modified)"
			}
			t.Errorf("In environment %v%s, %v.Path() = %q; want %q", test.env, euidMod, test.dir, path, test.path)
		}
	}
}

func TestDir_SearchPaths(t *testing.T) {
	td := newTempDir(t)
	defer td.cleanup()
	allopenDir := td.mkdir("allopen", 0777)
	secureDir := td.mkdir("secure", 0700)

	tests := []struct {
		dir   Dir
		env   env
		paths []string
	}{
		{
			dir:   Data,
			env:   env{},
			paths: []string{"/usr/local/share", "/usr/share"},
		},
		{
			dir:   Data,
			env:   env{"HOME": "/xHOMEx/me"},
			paths: []string{"/xHOMEx/me/.local/share", "/usr/local/share", "/usr/share"},
		},
		{
			dir:   Data,
			env:   env{"HOME": "/xHOMEx/me", "XDG_DATA_HOME": "/foo/data"},
			paths: []string{"/foo/data", "/usr/local/share", "/usr/share"},
		},
		{
			dir:   Data,
			env:   env{"XDG_DATA_HOME": "/foo/data", "XDG_DATA_DIRS": "/mybacon/data"},
			paths: []string{"/foo/data", "/mybacon/data"},
		},
		{
			dir:   Data,
			env:   env{"XDG_DATA_HOME": "/foo/data", "XDG_DATA_DIRS": "/mybacon/data:/eggs/data"},
			paths: []string{"/foo/data", "/mybacon/data", "/eggs/data"},
		},
		{
			dir:   Data,
			env:   env{"XDG_DATA_HOME": "/foo/data", "XDG_DATA_DIRS": "/mybacon/data:/eggs/data:/woka/woka"},
			paths: []string{"/foo/data", "/mybacon/data", "/eggs/data", "/woka/woka"},
		},
		{
			dir:   Data,
			env:   env{"XDG_DATA_HOME": "/foo/data", "XDG_DATA_DIRS": "/mybacon/data:relative/path:/woka/woka"},
			paths: []string{"/foo/data", "/mybacon/data", "/woka/woka"},
		},
		{
			dir:   Data,
			env:   env{"XDG_DATA_HOME": "relative/path", "XDG_DATA_DIRS": "/mybacon/data:relative/path:/woka/woka"},
			paths: []string{"/mybacon/data", "/woka/woka"},
		},
		{
			dir:   Data,
			env:   env{"XDG_DATA_DIRS": "/mybacon/data:/eggs/data:/woka/woka"},
			paths: []string{"/mybacon/data", "/eggs/data", "/woka/woka"},
		},
		{
			dir:   Config,
			env:   env{"XDG_CONFIG_HOME": "/foo/config", "XDG_CONFIG_DIRS": "/mybacon/config:/eggs/config:/woka/woka"},
			paths: []string{"/foo/config", "/mybacon/config", "/eggs/config", "/woka/woka"},
		},
		{
			// Cache only has primary dir
			dir:   Cache,
			env:   env{"XDG_CACHE_HOME": "/foo/cache", "XDG_CACHE_DIRS": "/mybacon/config:/eggs/config:/woka/woka"},
			paths: []string{"/foo/cache"},
		},
		{
			dir:   Runtime,
			env:   env{"XDG_RUNTIME_DIR": secureDir},
			paths: []string{secureDir},
		},
		{
			dir:   Runtime,
			env:   env{"XDG_RUNTIME_DIR": allopenDir},
			paths: []string{},
		},
	}
	for _, test := range tests {
		test.env.set()
		paths := test.dir.SearchPaths()
		if !stringsEqual(paths, test.paths) {
			t.Errorf("In environment %v, %v.SearchPaths() = %q; want %q", test.env, test.dir, paths, test.paths)
		}
	}
}

func TestDir_Open(t *testing.T) {
	td := newTempDir(t)
	defer td.cleanup()
	junkDir := td.mkdir("junk", 0777)
	dir1 := td.mkdir("dir1", 0777)
	dir2 := td.mkdir("dir2", 0777)
	dir3 := td.mkdir("dir3", 0777)
	td.newFile("dir1/foo.txt", "foo")
	td.newFile("dir1/multiple.txt", "1")
	td.newFile("dir2/bar.txt", "bar")
	td.newFile("dir2/only2_3.txt", "this is 2")
	td.newFile("dir2/multiple.txt", "2")
	td.newFile("dir3/multiple.txt", "3")
	td.newFile("dir3/only2_3.txt", "this is 3")

	tests := []struct {
		dir  Dir
		env  env
		name string

		path string
		err  bool
	}{
		{
			dir:  Data,
			env:  env{},
			name: "foo.txt",
			err:  true,
		},
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": dir1, "XDG_DATA_DIRS": junkDir},
			name: "foo.txt",
			path: filepath.Join(dir1, "foo.txt"),
		},
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": junkDir, "XDG_DATA_DIRS": junkDir},
			name: "foo.txt",
			err:  true,
		},
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": dir1, "XDG_DATA_DIRS": dir2},
			name: "foo.txt",
			path: filepath.Join(dir1, "foo.txt"),
		},
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": dir1, "XDG_DATA_DIRS": dir2},
			name: "bar.txt",
			path: filepath.Join(dir2, "bar.txt"),
		},
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": dir1, "XDG_DATA_DIRS": dir2 + ":" + dir3},
			name: "NOTREAL.txt",
			err:  true,
		},
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": dir1, "XDG_DATA_DIRS": dir2 + ":" + dir3},
			name: "foo.txt",
			path: filepath.Join(dir1, "foo.txt"),
		},
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": dir1, "XDG_DATA_DIRS": dir2 + ":" + dir3},
			name: "bar.txt",
			path: filepath.Join(dir2, "bar.txt"),
		},
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": dir1, "XDG_DATA_DIRS": dir2 + ":" + dir3},
			name: "multiple.txt",
			path: filepath.Join(dir1, "multiple.txt"),
		},
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": dir1, "XDG_DATA_DIRS": dir2 + ":" + dir3},
			name: "only2_3.txt",
			path: filepath.Join(dir2, "only2_3.txt"),
		},
	}
	for _, test := range tests {
		test.env.set()
		f, err := test.dir.Open(test.name)
		switch {
		case err == nil && test.err:
			t.Errorf("In environment %v, %v.Open(%q) succeeded; want error", test.env, test.dir, test.name)
		case err == nil && !test.err && f.Name() != test.path:
			t.Errorf("In environment %v, %v.Open(%q).Name() = %q; want %q", test.env, test.dir, test.name, f.Name(), test.path)
		case err != nil && !test.err:
			t.Errorf("In environment %v, %v.Open(%q) error: %v", test.env, test.dir, test.name, err)
		}
		if f != nil {
			f.Close()
		}
	}
}

func TestDir_Create(t *testing.T) {
	td := newTempDir(t)
	defer td.cleanup()
	junkDir := td.mkdir("junk", 0777)
	dataDir := td.mkdir("data", 0777)

	tests := []struct {
		dir  Dir
		env  env
		name string

		path       string
		err        bool
		permChecks []permCheck
	}{
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": dataDir, "XDG_DATA_DIRS": junkDir},
			name: "foo01",
			path: filepath.Join(dataDir, "foo01"),
		},
		{
			dir:  Data,
			env:  env{},
			name: "foo02",
			err:  true,
		},
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": dataDir, "XDG_DATA_DIRS": junkDir},
			name: filepath.Join("foo03", "bar"),
			path: filepath.Join(dataDir, "foo03", "bar"),
			permChecks: []permCheck{
				{filepath.Join(dataDir, "foo03"), 0700},
			},
		},
		{
			dir:  Data,
			env:  env{"XDG_DATA_HOME": filepath.Join(td.dir, "NOTREAL"), "XDG_DATA_DIRS": junkDir},
			name: filepath.Join("foo04", "bar"),
			path: filepath.Join(td.dir, "NOTREAL", "foo04", "bar"),
			permChecks: []permCheck{
				{filepath.Join(td.dir, "NOTREAL"), 0700},
				{filepath.Join(td.dir, "NOTREAL", "foo04"), 0700},
			},
		},
	}
	for _, test := range tests {
		test.env.set()
		f, err := test.dir.Create(test.name)
		switch {
		case err == nil && test.err:
			t.Errorf("In environment %v, %v.Create(%q) succeeded; want error", test.env, test.dir, test.name)
		case err == nil && !test.err && f.Name() != test.path:
			t.Errorf("In environment %v, %v.Create(%q).Name() = %q; want %q", test.env, test.dir, test.name, f.Name(), test.path)
		case err != nil && !test.err:
			t.Errorf("In environment %v, %v.Create(%q) error: %v", test.env, test.dir, test.name, err)
		}
		if f != nil {
			f.Close()
		}
		for _, pc := range test.permChecks {
			info, err := os.Stat(pc.name)
			if err != nil {
				t.Errorf("In environment %v, %v.Create(%q): stat %s error: %v", test.env, test.dir, test.name, pc.name, err)
				continue
			}
			if perm := info.Mode().Perm(); perm != pc.perm {
				t.Errorf("In environment %v, %v.Create(%q): %s has permission %v; want %v", test.env, test.dir, test.name, pc.name, perm, pc.perm)
			}
		}
	}
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type tempDir struct {
	t   *testing.T
	dir string
}

func newTempDir(t *testing.T) *tempDir {
	td := &tempDir{t: t}
	var err error
	td.dir, err = ioutil.TempDir("", "xdgdir_test")
	if err != nil {
		t.Fatal("making temp dir:", err)
	}
	return td
}

// newFile creates a file and returns its path.
func (td *tempDir) newFile(name string, data string) string {
	path := filepath.Join(td.dir, name)
	f, err := os.Create(path)
	if err != nil {
		td.t.Fatalf("newFile(%q, %q) error: %v", name, data, err)
	}
	_, werr := f.Write([]byte(data))
	cerr := f.Close()
	if werr != nil {
		td.t.Errorf("newFile(%q, %q) write error: %v", name, data, err)
	}
	if cerr != nil {
		td.t.Errorf("newFile(%q, %q) close error: %v", name, data, err)
	}
	if werr != nil || cerr != nil {
		td.t.FailNow()
	}
	return path
}

// mkdir creates a directory and returns its path.
func (td *tempDir) mkdir(name string, perm os.FileMode) string {
	path := filepath.Join(td.dir, name)
	err := os.Mkdir(path, perm)
	if err != nil {
		td.t.Fatal(err)
	}
	return path
}

func (td *tempDir) cleanup() {
	err := os.RemoveAll(td.dir)
	if err != nil {
		td.t.Log("failed to clean up temp dir:", err)
	}
}

type permCheck struct {
	name string
	perm os.FileMode
}

type env map[string]string

func (e env) set() {
	getenv = func(key string) string {
		return e[key]
	}
	findHome = func() string {
		return e["HOME"]
	}
}
