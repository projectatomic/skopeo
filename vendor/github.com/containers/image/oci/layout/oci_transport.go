package layout

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/containers/image/directory/explicitfilepath"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/image"
	"github.com/containers/image/types"
)

// Transport is an ImageTransport for OCI directories.
var Transport = ociTransport{}

type ociTransport struct{}

func (t ociTransport) Name() string {
	return "oci"
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an ImageReference.
func (t ociTransport) ParseReference(reference string) (types.ImageReference, error) {
	return ParseReference(reference)
}

var refRegexp = regexp.MustCompile(`^([A-Za-z0-9._-]+)+$`)

// ValidatePolicyConfigurationScope checks that scope is a valid name for a signature.PolicyTransportScopes keys
// (i.e. a valid PolicyConfigurationIdentity() or PolicyConfigurationNamespaces() return value).
// It is acceptable to allow an invalid value which will never be matched, it can "only" cause user confusion.
// scope passed to this function will not be "", that value is always allowed.
func (t ociTransport) ValidatePolicyConfigurationScope(scope string) error {
	var dir string
	sep := strings.LastIndex(scope, ":")
	if sep == -1 {
		dir = scope
	} else {
		dir = scope[:sep]
		tag := scope[sep+1:]
		if !refRegexp.MatchString(tag) {
			return fmt.Errorf("Invalid tag %s", tag)
		}
	}

	if strings.Contains(dir, ":") {
		return fmt.Errorf("Invalid OCI reference %s: path contains a colon", scope)
	}

	if !strings.HasPrefix(dir, "/") {
		return fmt.Errorf("Invalid scope %s: must be an absolute path", scope)
	}
	// Refuse also "/", otherwise "/" and "" would have the same semantics,
	// and "" could be unexpectedly shadowed by the "/" entry.
	// (Note: we do allow "/:sometag", a bit ridiculous but why refuse it?)
	if scope == "/" {
		return errors.New(`Invalid scope "/": Use the generic default scope ""`)
	}
	cleaned := filepath.Clean(dir)
	if cleaned != dir {
		return fmt.Errorf(`Invalid scope %s: Uses non-canonical path format, perhaps try with path %s`, scope, cleaned)
	}
	return nil
}

// ociReference is an ImageReference for OCI directory paths.
type ociReference struct {
	// Note that the interpretation of paths below depends on the underlying filesystem state, which may change under us at any time!
	// Either of the paths may point to a different, or no, inode over time.  resolvedDir may contain symbolic links, and so on.

	// Generally we follow the intent of the user, and use the "dir" member for filesystem operations (e.g. the user can use a relative path to avoid
	// being exposed to symlinks and renames in the parent directories to the working directory).
	// (But in general, we make no attempt to be completely safe against concurrent hostile filesystem modifications.)
	dir         string // As specified by the user. May be relative, contain symlinks, etc.
	resolvedDir string // Absolute path with no symlinks, at least at the time of its creation. Primarily used for policy namespaces.
	tag         string
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an OCI ImageReference.
func ParseReference(reference string) (types.ImageReference, error) {
	var dir, tag string
	sep := strings.LastIndex(reference, ":")
	if sep == -1 {
		dir = reference
		tag = "latest"
	} else {
		dir = reference[:sep]
		tag = reference[sep+1:]
	}
	return NewReference(dir, tag)
}

// NewReference returns an OCI reference for a directory and a tag.
//
// We do not expose an API supplying the resolvedDir; we could, but recomputing it
// is generally cheap enough that we prefer being confident about the properties of resolvedDir.
func NewReference(dir, tag string) (types.ImageReference, error) {
	resolved, err := explicitfilepath.ResolvePathToFullyExplicit(dir)
	if err != nil {
		return nil, err
	}
	// This is necessary to prevent directory paths returned by PolicyConfigurationNamespaces
	// from being ambiguous with values of PolicyConfigurationIdentity.
	if strings.Contains(resolved, ":") {
		return nil, fmt.Errorf("Invalid OCI reference %s:%s: path %s contains a colon", dir, tag, resolved)
	}
	if !refRegexp.MatchString(tag) {
		return nil, fmt.Errorf("Invalid tag %s", tag)
	}
	return ociReference{dir: dir, resolvedDir: resolved, tag: tag}, nil
}

func (ref ociReference) Transport() types.ImageTransport {
	return Transport
}

// StringWithinTransport returns a string representation of the reference, which MUST be such that
// reference.Transport().ParseReference(reference.StringWithinTransport()) returns an equivalent reference.
// NOTE: The returned string is not promised to be equal to the original input to ParseReference;
// e.g. default attribute values omitted by the user may be filled in in the return value, or vice versa.
// WARNING: Do not use the return value in the UI to describe an image, it does not contain the Transport().Name() prefix.
func (ref ociReference) StringWithinTransport() string {
	return fmt.Sprintf("%s:%s", ref.dir, ref.tag)
}

// DockerReference returns a Docker reference associated with this reference
// (fully explicit, i.e. !reference.IsNameOnly, but reflecting user intent,
// not e.g. after redirect or alias processing), or nil if unknown/not applicable.
func (ref ociReference) DockerReference() reference.Named {
	return nil
}

// PolicyConfigurationIdentity returns a string representation of the reference, suitable for policy lookup.
// This MUST reflect user intent, not e.g. after processing of third-party redirects or aliases;
// The value SHOULD be fully explicit about its semantics, with no hidden defaults, AND canonical
// (i.e. various references with exactly the same semantics should return the same configuration identity)
// It is fine for the return value to be equal to StringWithinTransport(), and it is desirable but
// not required/guaranteed that it will be a valid input to Transport().ParseReference().
// Returns "" if configuration identities for these references are not supported.
func (ref ociReference) PolicyConfigurationIdentity() string {
	return fmt.Sprintf("%s:%s", ref.resolvedDir, ref.tag)
}

// PolicyConfigurationNamespaces returns a list of other policy configuration namespaces to search
// for if explicit configuration for PolicyConfigurationIdentity() is not set.  The list will be processed
// in order, terminating on first match, and an implicit "" is always checked at the end.
// It is STRONGLY recommended for the first element, if any, to be a prefix of PolicyConfigurationIdentity(),
// and each following element to be a prefix of the element preceding it.
func (ref ociReference) PolicyConfigurationNamespaces() []string {
	res := []string{}
	path := ref.resolvedDir
	for {
		lastSlash := strings.LastIndex(path, "/")
		// Note that we do not include "/"; it is redundant with the default "" global default,
		// and rejected by ociTransport.ValidatePolicyConfigurationScope above.
		if lastSlash == -1 || path == "/" {
			break
		}
		res = append(res, path)
		path = path[:lastSlash]
	}
	return res
}

// NewImage returns a types.Image for this reference, possibly specialized for this ImageTransport.
// The caller must call .Close() on the returned Image.
// NOTE: If any kind of signature verification should happen, build an UnparsedImage from the value returned by NewImageSource,
// verify that UnparsedImage, and convert it into a real Image via image.FromUnparsedImage.
func (ref ociReference) NewImage(ctx *types.SystemContext) (types.Image, error) {
	src := newImageSource(ref)
	return image.FromSource(src)
}

// NewImageSource returns a types.ImageSource for this reference,
// asking the backend to use a manifest from requestedManifestMIMETypes if possible.
// nil requestedManifestMIMETypes means manifest.DefaultRequestedManifestMIMETypes.
// The caller must call .Close() on the returned ImageSource.
func (ref ociReference) NewImageSource(ctx *types.SystemContext, requestedManifestMIMETypes []string) (types.ImageSource, error) {
	return newImageSource(ref), nil
}

// NewImageDestination returns a types.ImageDestination for this reference.
// The caller must call .Close() on the returned ImageDestination.
func (ref ociReference) NewImageDestination(ctx *types.SystemContext) (types.ImageDestination, error) {
	return newImageDestination(ref), nil
}

// DeleteImage deletes the named image from the registry, if supported.
func (ref ociReference) DeleteImage(ctx *types.SystemContext) error {
	return fmt.Errorf("Deleting images not implemented for oci: images")
}

// ociLayoutPathPath returns a path for the oci-layout within a directory using OCI conventions.
func (ref ociReference) ociLayoutPath() string {
	return filepath.Join(ref.dir, "oci-layout")
}

// blobPath returns a path for a blob within a directory using OCI image-layout conventions.
func (ref ociReference) blobPath(digest string) (string, error) {
	pts := strings.SplitN(digest, ":", 2)
	if len(pts) != 2 {
		return "", fmt.Errorf("unexpected digest reference %s", digest)
	}
	return filepath.Join(ref.dir, "blobs", pts[0], pts[1]), nil
}

// descriptorPath returns a path for the manifest within a directory using OCI conventions.
func (ref ociReference) descriptorPath(digest string) string {
	return filepath.Join(ref.dir, "refs", digest)
}
