package image

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
)

type platformSpec struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
	Features     []string `json:"features,omitempty"`
}

// A manifestDescriptor references a platform-specific manifest.
type manifestDescriptor struct {
	descriptor
	Platform platformSpec `json:"platform"`
}

type manifestList struct {
	SchemaVersion int                  `json:"schemaVersion"`
	MediaType     string               `json:"mediaType"`
	Manifests     []manifestDescriptor `json:"manifests"`
}

func manifestSchema2FromManifestList(src types.ImageSource, manblob []byte) (genericManifest, error) {
	list := manifestList{}
	if err := json.Unmarshal(manblob, &list); err != nil {
		return nil, err
	}
	var targetManifestDigest string
	for _, d := range list.Manifests {
		if d.Platform.Architecture == runtime.GOARCH && d.Platform.OS == runtime.GOOS {
			targetManifestDigest = d.Digest
			break
		}
	}
	if targetManifestDigest == "" {
		return nil, errors.New("no supported platform found in manifest list")
	}
	manblob, mt, err := src.GetTargetManifest(targetManifestDigest)
	if err != nil {
		return nil, err
	}

	matches, err := manifest.MatchesDigest(manblob, targetManifestDigest)
	if err != nil {
		return nil, fmt.Errorf("Error computing manifest digest: %v", err)
	}
	if !matches {
		return nil, fmt.Errorf("Manifest image does not match selected manifest digest %s", targetManifestDigest)
	}

	return manifestInstanceFromBlob(src, manblob, mt)
}
