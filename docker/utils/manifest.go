package utils

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
)

// GuessManifestMIMEType guesses MIME type of a manifest and returns it _if it is recognized_, or "" if unknown or unrecognized.
// FIXME? We should, in general, prefer out-of-band MIME type instead of blindly parsing the manifest,
// but we may not have such metadata available (e.g. when the manifest is a local file).
func GuessManifestMIMEType(manifest []byte) string {
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
	case DockerV2Schema2MIMEType, DockerV2ListMIMEType: // A recognized type.
		return meta.MediaType
	}
	switch meta.SchemaVersion {
	case 1:
		return DockerV2Schema1MIMEType
	case 2: // Really should not happen, meta.MediaType should have been set. But given the data, this is our best guess.
		return DockerV2Schema2MIMEType
	}
	return ""
}

// ManifestDigest returns the a digest of a docker manifest, with any necessary implied transformations like stripping v1s1 signatures.
func ManifestDigest(manifest []byte) (string, error) {
	if GuessManifestMIMEType(manifest) == DockerV2Schema1MIMEType {
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
