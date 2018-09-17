/*
Copyright 2018 The go4 Authors

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

// NewBufferingReaderAt returns an io.ReaderAt that reads from r as
// necessary and keeps a copy of all data read in memory.
func NewBufferingReaderAt(r io.Reader) io.ReaderAt {
	return &bufReaderAt{r: r}
}

type bufReaderAt struct {
	r   io.Reader
	buf []byte
}

func (br *bufReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	endOff := off + int64(len(p))
	need := endOff - int64(len(br.buf))
	if need > 0 {
		buf := make([]byte, need)
		var rn int
		rn, err = io.ReadFull(br.r, buf)
		br.buf = append(br.buf, buf[:rn]...)
	}
	if int64(len(br.buf)) >= off {
		n = copy(p, br.buf[off:])
	}
	if n == len(p) {
		err = nil
	}
	return
}
