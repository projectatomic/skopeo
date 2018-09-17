/*
Copyright 2012 The Go4 Authors

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

package readerutil

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

const text = "HelloWorld"

type testSrc struct {
	name string
	src  io.Reader
	want int64
}

func (tsrc *testSrc) run(t *testing.T) {
	n, ok := Size(tsrc.src)
	if !ok {
		t.Fatalf("failed to read size for %q", tsrc.name)
	}
	if n != tsrc.want {
		t.Fatalf("wanted %v, got %v", tsrc.want, n)
	}
}

func TestBytesBuffer(t *testing.T) {
	buf := bytes.NewBuffer([]byte(text))
	tsrc := &testSrc{"buffer", buf, int64(len(text))}
	tsrc.run(t)
}

func TestSeeker(t *testing.T) {
	f, err := ioutil.TempFile("", "camliTestReaderSize")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	size, err := f.Write([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	pos, err := f.Seek(5, 0)
	if err != nil {
		t.Fatal(err)
	}
	tsrc := &testSrc{"seeker", f, int64(size) - pos}
	tsrc.run(t)
}
