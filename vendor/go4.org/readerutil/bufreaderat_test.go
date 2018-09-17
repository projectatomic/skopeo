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

import "testing"

type trackingReader struct {
	off       int
	reads     int
	readBytes int
}

func (t *trackingReader) Read(p []byte) (n int, err error) {
	t.reads++
	t.readBytes += len(p)
	for len(p) > 0 {
		p[0] = '0' + byte(t.off%10)
		t.off++
		p = p[1:]
		n++
	}
	return

}

func TestBufferingReaderAt(t *testing.T) {
	tr := new(trackingReader)
	ra := NewBufferingReaderAt(tr)
	for i, tt := range []struct {
		off           int64
		want          string
		wantReads     int
		wantReadBytes int
	}{
		{off: 0, want: "0123456789", wantReads: 1, wantReadBytes: 10},
		{off: 5, want: "56789", wantReads: 1, wantReadBytes: 10},      // already buffered
		{off: 6, want: "67890", wantReads: 2, wantReadBytes: 11},      // need 1 more byte
		{off: 0, want: "0123456789", wantReads: 2, wantReadBytes: 11}, // already buffered
	} {
		got := make([]byte, len(tt.want))
		n, err := ra.ReadAt(got, tt.off)
		if err != nil || n != len(tt.want) {
			t.Errorf("step %d: ReadAt = %v, %v; want %v, %v", i, n, err, len(tt.want), nil)
			continue
		}
		if string(got) != tt.want {
			t.Errorf("step %d: ReadAt read %q; want %q", i, got, tt.want)
		}
		if tr.reads != tt.wantReads {
			t.Errorf("step %d: num reads = %d; want %d", i, tr.reads, tt.wantReads)
		}
		if tr.readBytes != tt.wantReadBytes {
			t.Errorf("step %d: read bytes = %d; want %d", i, tr.reads, tt.wantReads)
		}
	}
}
