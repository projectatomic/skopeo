/*
Copyright 2015 The Perkeep Authors

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

package gcs

import (
	"bytes"
	"flag"
	"io"
	"strings"
	"testing"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/storage"
	"go4.org/wkfs"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
)

var flagBucket = flag.String("bucket", "", "Google Cloud Storage bucket where to run the tests. It should be empty.")

func TestWriteRead(t *testing.T) {
	if !metadata.OnGCE() {
		t.Skipf("Not testing on GCE")
	}
	if *flagBucket == "" {
		t.Skipf("No bucket specified")
	}
	ctx := context.Background()
	cl, err := storage.NewClient(ctx)
	it := cl.Bucket(*flagBucket).Objects(ctx, nil)
	if _, err := it.Next(); err != iterator.Done {
		if err == nil {
			t.Fatalf("Bucket %v is not empty, aborting test.", *flagBucket)
		}
		t.Fatalf("unexpected bucket iteration error: %v", err)
	}

	// Write to camli-gcs_test.txt
	filename := "camli-gcs_test.txt"
	gcsPath := "/gcs/" + *flagBucket + "/" + filename
	f, err := wkfs.Create(gcsPath)
	if err != nil {
		t.Fatalf("error creating %v: %v", gcsPath, err)
	}
	defer func() {
		if err := wkfs.Remove(gcsPath); err != nil {
			t.Fatalf("error while cleaning up %v: %v", gcsPath, err)
		}
	}()

	data := "Hello World"
	if _, err := io.Copy(f, strings.NewReader(data)); err != nil {
		t.Fatalf("error writing to %v: %v", gcsPath, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("error closing %v: %v", gcsPath, err)
	}

	// Read back from camli-gcs_test.txt
	g, err := wkfs.Open(gcsPath)
	if err != nil {
		t.Fatalf("error opening %v: %v", gcsPath, err)
	}
	defer g.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, g); err != nil {
		t.Fatalf("error reading %v: %v", gcsPath, err)
	}
	if buf.String() != data {
		t.Fatalf("error with %v contents: got %v, wanted %v", gcsPath, buf.String(), data)
	}
}
