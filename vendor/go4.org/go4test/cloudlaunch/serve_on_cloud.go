// +build ignore

/*
Copyright 2016 The Go4 Authors.

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

// The serve_on_cloud program deploys an HTTP server on Google Compute Engine,
// serving from Google Cloud Storage. Its purpose is to help testing
// go4.org/cloud/cloudlaunch and go4.org/wkfs/gcs.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"time"

	"go4.org/cloud/cloudlaunch"
	"go4.org/wkfs"
	_ "go4.org/wkfs/gcs"

	"cloud.google.com/go/compute/metadata"
	storageapi "google.golang.org/api/storage/v1"
	compute "google.golang.org/api/compute/v1"
)

var httpAddr = flag.String("http", ":80", "HTTP address")

var gcsBucket string

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	rc, err := wkfs.Open(path.Join("/gcs", gcsBucket, r.URL.Path))
	if err != nil {
		http.Error(w, fmt.Sprintf("could not open %v: %v", r.URL.Path, err), 500)
		return
	}
	defer rc.Close()
	http.ServeContent(w, r, r.URL.Path, time.Now(), rc)
}

func main() {
	if !metadata.OnGCE() {
		bucket := os.Getenv("GCSBUCKET")
		if bucket == "" {
			log.Fatal("You need to set the GCSBUCKET env var to specify the Google Cloud Storage bucket to serve from.")
		}
		projectID := os.Getenv("GCEPROJECTID")
		if projectID == "" {
			log.Fatal("You need to set the GCEPROJECTID env var to specify the Google Cloud project where the instance will run.")
		}
		(&cloudlaunch.Config{
			Name:         "serveoncloud",
			BinaryBucket: bucket,
			GCEProjectID: projectID,
			Scopes: []string{
				storageapi.DevstorageFullControlScope,
				compute.ComputeScope,
			},
		}).MaybeDeploy()
		return
	}

	flag.Parse()

	storageURLRxp := regexp.MustCompile(`https://storage.googleapis.com/(.+?)/serveoncloud.*`)
	cloudConfig, err := metadata.InstanceAttributeValue("user-data")
	if err != nil || cloudConfig == "" {
		log.Fatalf("could not get cloud config from metadata: %v", err)
	}
	m := storageURLRxp.FindStringSubmatch(cloudConfig)
	if len(m) < 2 {
		log.Fatal("storage URL not found in cloud config")
	}
	gcsBucket = m[1]

	http.HandleFunc("/", serveHTTP)

	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}
