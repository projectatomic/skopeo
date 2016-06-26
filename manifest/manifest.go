package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/docker/libtrust"
)

// FIXME: Should we just use docker/distribution and docker/docker implementations directly?

const (
	// DockerV2Schema1MIMEType MIME type represents Docker manifest schema 1
	DockerV2Schema1MIMEType = "application/vnd.docker.distribution.manifest.v1+json"
	// DockerV2Schema2MIMEType MIME type represents Docker manifest schema 2
	DockerV2Schema2MIMEType = "application/vnd.docker.distribution.manifest.v2+json"
	// DockerV2ListMIMEType MIME type represents Docker manifest schema 2 list
	DockerV2ListMIMEType = "application/vnd.docker.distribution.manifest.list.v2+json"

	// OCIV1DescriptorMIMEType TODO
	OCIV1DescriptorMIMEType = "application/vnd.oci.descriptor.v1+json"
	// OCIV1ImageManifestMIMEType TODO
	OCIV1ImageManifestMIMEType = "application/vnd.oci.image.manifest.v1+json"
	// OCIV1ImageManifestListMIMEType TODO
	OCIV1ImageManifestListMIMEType = "application/vnd.oci.image.manifest.list.v1+json"
	// OCIV1ImageSerializationRootfsTarGzipMIMEType TODO(unimplemented)
	OCIV1ImageSerializationRootfsTarGzipMIMEType = "application/vnd.oci.image.serialization.rootfs.tar.gzip"
	// OCIV1ImageSerializationConfigMIMEType TODO
	OCIV1ImageSerializationConfigMIMEType = "application/vnd.oci.image.serialization.config.v1+json"
)

// GuessMIMEType guesses MIME type of a manifest and returns it _if it is recognized_, or "" if unknown or unrecognized.
// FIXME? We should, in general, prefer out-of-band MIME type instead of blindly parsing the manifest,
// but we may not have such metadata available (e.g. when the manifest is a local file).
func GuessMIMEType(manifest []byte) string {
	// A subset of manifest fields; the rest is silently ignored by json.Unmarshal.
	// Also docker/distribution/manifest.Versioned.
	meta := struct {
		MediaType     string `json:"mediaType"`
		SchemaVersion int    `json:"schemaVersion"`
	}{}
	if err := json.Unmarshal(manifest, &meta); err != nil {
		return ""
	}

	switch meta.MediaType {
	case DockerV2Schema2MIMEType, DockerV2ListMIMEType, OCIV1ImageManifestMIMEType, OCIV1ImageManifestListMIMEType: // A recognized type.
		return meta.MediaType
	}
	// this is the only way the function can return DockerV2Schema1MIMEType, and recognizing that is essential for stripping the JWS signatures = computing the correct manifest digest.
	switch meta.SchemaVersion {
	case 1:
		return DockerV2Schema1MIMEType
	case 2: // Really should not happen, meta.MediaType should have been set. But given the data, this is our best guess.
		return DockerV2Schema2MIMEType
	}
	return ""
}

// Digest returns the a digest of a docker manifest, with any necessary implied transformations like stripping v1s1 signatures.
func Digest(manifest []byte) (string, error) {
	if GuessMIMEType(manifest) == DockerV2Schema1MIMEType {
		sig, err := libtrust.ParsePrettySignature(manifest, "signatures")
		if err != nil {
			return "", err
		}
		manifest, err = sig.Payload()
		if err != nil {
			// Coverage: This should never happen, libtrust's Payload() can fail only if joseBase64UrlDecode() fails, on a string
			// that libtrust itself has josebase64UrlEncode()d
			return "", err
		}
	}

	hash := sha256.Sum256(manifest)
	return "sha256:" + hex.EncodeToString(hash[:]), nil
}

// MatchesDigest returns true iff the manifest matches expectedDigest.
// Error may be set if this returns false.
// Note that this is not doing ConstantTimeCompare; by the time we get here, the cryptographic signature must already have been verified,
// or we are not using a cryptographic channel and the attacker can modify the digest along with the manifest blob.
func MatchesDigest(manifest []byte, expectedDigest string) (bool, error) {
	// This should eventually support various digest types.
	actualDigest, err := Digest(manifest)
	if err != nil {
		return false, err
	}
	return expectedDigest == actualDigest, nil
}
