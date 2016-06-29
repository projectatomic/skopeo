package types

import (
	"io"
	"time"
)

// Registry is a service providing repositories.
type Registry interface {
	Repositories() []Repository
	Repository(ref string) Repository
	Lookup(term string) []Image // docker registry v1 only AFAICT, v2 can be built hacking with Images()
}

// Repository is a set of images.
type Repository interface {
	Images() []Image
	Image(ref string) Image // ref == image name w/o registry part
}

// ImageSource is a service, possibly remote (= slow), to download components of a single image.
// This is primarily useful for copying images around; for examining their properties, Image (below)
// is usually more useful.
type ImageSource interface {
	// IntendedDockerReference returns the full, unambiguous, Docker reference for this image, _as specified by the user_
	// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
	// May be "" if unknown.
	IntendedDockerReference() string
	// GetManifest returns the image's manifest along with its MIME type. The empty string is returned if the MIME type is unknown. The slice parameter indicates the supported mime types the manifest should be when getting it.
	// It may use a remote (= slow) service.
	GetManifest([]string) ([]byte, string, error)
	// Note: Calling GetBlob() may have ordering dependencies WRT other methods of this type. FIXME: How does this work with (docker save) on stdin?
	// the second return value is the size of the blob. If not known 0 is returned
	GetBlob(digest string) (io.ReadCloser, int64, error)
	// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
	GetSignatures() ([][]byte, error)
	// Delete image from registry, if operation is supported
	Delete() error
}

// ImageDestination is a service, possibly remote (= slow), to store components of a single image.
type ImageDestination interface {
	// CanonicalDockerReference returns the full, unambiguous, Docker reference for this image (even if the user referred to the image using some shorthand notation).
	CanonicalDockerReference() (string, error)
	// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
	PutManifest([]byte) error
	// Note: Calling PutBlob() and other methods may have ordering dependencies WRT other methods of this type. FIXME: Figure out and document.
	PutBlob(digest string, stream io.Reader) error
	PutSignatures(signatures [][]byte) error
	SupportedImageDestinationMIMEType() []string
}

// Image is the primary API for inspecting properties of images.
type Image interface {
	// ref to repository?
	// IntendedDockerReference returns the full, unambiguous, Docker reference for this image, _as specified by the user_
	// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
	// May be "" if unknown.
	IntendedDockerReference() string
	// Manifest is like ImageSource.GetManifest, but the result is cached; it is OK to call this however often you need.
	// NOTE: It is essential for signature verification that Manifest returns the manifest from which LayerDigests is computed.
	Manifest() ([]byte, error)
	// Signatures is like ImageSource.GetSignatures, but the result is cached; it is OK to call this however often you need.
	Signatures() ([][]byte, error)
	// LayerDigests returns a list of layer digests referenced by this image.
	// The list will not contain duplicates; it is not intended to correspond to the "history" or "parent chain" of a Docker image.
	// NOTE: It is essential for signature verification that LayerDigests is computed from the same manifest which is returned by Manifest().
	LayerDigests() ([]string, error)
	// LayersCommand implements (skopeo layers).  Do not use for any other purpose.
	// Longer-term we would like to move the command-specific code up to the command handler,
	// but the command has functionality specific to util.DockerV2Schema1MIMEType manifests.
	LayersCommand(layers ...string) error // configure download directory?
	// Inspect returns various information for (skopeo inspect) parsed from the manifest and configuration.
	Inspect() (*ImageInspectInfo, error)
	DockerTar() ([]byte, error) // ??? also, configure output directory
}

// ImageInspectInfo is a set of metadata describing Docker images, primarily their manifest and configuration.
// The Tag field is a legacy field which is here just for the Docker v2s1 manifest. It won't be supported
// for other manifest types.
type ImageInspectInfo struct {
	Tag           string
	Created       time.Time
	DockerVersion string
	Labels        map[string]string
	Architecture  string
	Os            string
	Layers        []string
}
