package docker

import "github.com/containers/image/docker/reference"

// parseDockerImageName converts a string into a reference and tag value.
func parseDockerImageName(img string) (reference.Named, string, error) {
	ref, err := reference.ParseNamed(img)
	if err != nil {
		return nil, "", err
	}
	ref = reference.WithDefaultTag(ref)
	var tag string
	switch x := ref.(type) {
	case reference.Canonical:
		tag = x.Digest().String()
	case reference.NamedTagged:
		tag = x.Tag()
	}
	return ref, tag, nil
}
