package skopeo

import (
	"fmt"
	"strings"

	"github.com/projectatomic/skopeo/types"
)

// ParseImage converts image URL-like string to an initialized handler for that image.
func ParseImage(img string) (types.Image, error) {
	switch {
	case strings.HasPrefix(img, types.DockerPrefix):
		return parseDockerImage(strings.TrimPrefix(img, dockerPrefix))
		//case strings.HasPrefix(img, appcPrefix):
		//
	}
	return nil, fmt.Errorf("no valid prefix provided")
}
