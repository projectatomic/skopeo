/*
Copyright 2011 The Go4 Authors

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

import "io"

// CountingReader wraps a Reader, incrementing N by the number of
// bytes read. No locking is performed.
type CountingReader struct {
	Reader io.Reader
	N      *int64
}

func (cr CountingReader) Read(p []byte) (n int, err error) {
	n, err = cr.Reader.Read(p)
	*cr.N += int64(n)
	return
}
