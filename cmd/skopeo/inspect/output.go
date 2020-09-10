package inspect

import (
	"time"

	digest "github.com/opencontainers/go-digest"
)

// Output is the output format of (skopeo inspect),
// primarily so that we can format it with a simple json.MarshalIndent.
type Output struct {
	Name          string `json:",omitempty"`
	Tag           string `json:",omitempty"`
	Digest        digest.Digest
	Signers       []string `json:",omitempty"`
	RepoTags      []string
	Created       *time.Time
	DockerVersion string
	Labels        map[string]string
	Architecture  string
	Os            string
	Layers        []string
	Env           []string
}
