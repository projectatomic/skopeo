/*
Copyright 2013 The Go4 Authors

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

package singlereader

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"testing"
)

func TestOpenSingle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))
	f, err := ioutil.TempFile("", "foo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	contents := []byte("Some file contents")
	if _, err := f.Write(contents); err != nil {
		t.Fatal(err)
	}
	f.Close()

	const j = 4
	errc := make(chan error, j)
	for i := 1; i < j; i++ {
		go func() {
			buf := make([]byte, len(contents))
			for i := 0; i < 400; i++ {
				rac, err := Open(f.Name())
				if err != nil {
					errc <- err
					return
				}
				n, err := rac.ReadAt(buf, 0)
				if err != nil {
					errc <- err
					return
				}
				if n != len(contents) || !bytes.Equal(buf, contents) {
					errc <- fmt.Errorf("read %d, %q; want %d, %q", n, buf, len(contents), contents)
					return
				}
				if err := rac.Close(); err != nil {
					errc <- err
					return
				}
			}
			errc <- nil
		}()
	}
	for i := 1; i < j; i++ {
		if err := <-errc; err != nil {
			t.Error(err)
		}
	}
}
