// Note: Consider the API unstable until the code supports at least three different image formats or transports.

// This defines types used to represent a signature verification policy in memory.
// Do not use the private types directly; either parse a configuration file, or construct a Policy from PolicyRequirements
// built using the constructor functions provided in policy_config.go.

package signature

// Policy defines requirements for considering a signature valid.
type Policy struct {
	// Default applies to any image which does not have a matching policy in Specific,
	// and to images with unspecified identity, e.g. local directories (dir:…).
	Default PolicyRequirements `json:"default"`
	// Specific applies to images matching scope, the map key.
	// Scope is hostname[/zero/or/more/namespaces[/repository[:tag|@digest]]]; note that in order to be
	// unambiguous, this must use a fully expanded format, e.g. "docker.io/library/busybox" or
	// "docker.io/library", not "busybox" or "library".
	// FIXME: Scope syntax - should it be namespaced docker:something ? Or, in the worst case, a composite object (we couldn't use a JSON map)
	// Most specific scope wins, duplication is prohibited (hard failure).
	// Defaults to an empty map if not specified.
	Specific map[string]PolicyRequirements `json:"specific"`
}

// PolicyRequirements is a set of requirements applying to a set of images; each of them must be satisfied (though perhaps each by a different signature).
// Must not be empty, frequently will only contain a single element.
type PolicyRequirements []PolicyRequirement

// PolicyRequirement is a rule which must be satisfied by at least one of the signatures of an image.
// The type is public, but its definition is private.

// prCommon is the common type field in a JSON encoding of PolicyRequirement.
type prCommon struct {
	Type prTypeIdentifier `json:"type"`
}

// prTypeIdentifier is string designating a kind of a PolicyRequirement.
type prTypeIdentifier string

const (
	prTypeInsecureAcceptAnything prTypeIdentifier = "insecureAcceptAnything"
	prTypeReject                 prTypeIdentifier = "reject"
	prTypeSignedBy               prTypeIdentifier = "signedBy"
	prTypeSignedBaseLayer        prTypeIdentifier = "signedBaseLayer"
)

// prInsecureAcceptAnything is a PolicyRequirement with type = prTypeInsecureAcceptAnything:
// every image is allowed to run.
// Note that because PolicyRequirements are implicitly ANDed, this is necessary only if it is the only rule (to make the list non-empty and the policy explicit).
// NOTE: This allows the image to run; it DOES NOT consider the signature verified (per IsSignatureAuthorAccepted).
// FIXME? Better name?
type prInsecureAcceptAnything struct {
	prCommon
}

// prReject is a PolicyRequirement with type = prTypeReject: every image is rejected.
type prReject struct {
	prCommon
}

// prSignedBy is a PolicyRequirement with type = prTypeSignedBy: the image is signed by trusted keys for a specified identity
type prSignedBy struct {
	prCommon

	// KeyType specifies what kind of key reference KeyPath/KeyData is.
	// Acceptable values are “GPGKeys” | “signedByGPGKeys” “X.509Certificates” | “signedByX.509CAs”
	// FIXME: eventually also support GPGTOFU, X.509TOFU, with KeyPath only
	KeyType sbKeyType `json:"keyType"`

	// KeyPath is a pathname to a local file containing the trusted key(s). Exactly one of KeyPath and KeyData must be specified.
	KeyPath string `json:"keyPath,omitempty"`
	// KeyData contains the trusted key(s), base64-encoded. Exactly one of KeyPath and KeyData must be specified.
	KeyData []byte `json:"keyData,omitempty"`

	// SignedIdentity specifies what image identity the signature must be claiming about the image.
	// Defaults to "match-exact" if not specified.
	SignedIdentity PolicyReferenceMatch `json:"signedIdentity"`
}

// sbKeyType are the allowed values for prSignedBy.KeyType
type sbKeyType string

const (
	// SBKeyTypeGPGKeys refers to keys contained in a GPG keyring
	SBKeyTypeGPGKeys sbKeyType = "GPGKeys"
	// SBKeyTypeSignedByGPGKeys refers to keys signed by keys in a GPG keyring
	SBKeyTypeSignedByGPGKeys sbKeyType = "signedByGPGKeys"
	// SBKeyTypeX509Certificates refers to keys in a set of X.509 certificates
	// FIXME: PEM, DER?
	SBKeyTypeX509Certificates sbKeyType = "X509Certificates"
	// SBKeyTypeSignedByX509CAs refers to keys signed by one of the X.509 CAs
	// FIXME: PEM, DER?
	SBKeyTypeSignedByX509CAs sbKeyType = "signedByX509CAs"
)

// prSignedBaseLayer is a PolicyRequirement with type = prSignedBaseLayer: the image has a specified, correctly signed, base image.
type prSignedBaseLayer struct {
	prCommon
	// BaseLayerIdentity specifies the base image to look for. "match-exact" is rejected, "match-repository" is unlikely to be useful.
	BaseLayerIdentity PolicyReferenceMatch `json:"baseLayerIdentity"`
}

// PolicyReferenceMatch specifies a set of image identities accepted in PolicyRequirement.
// The type is public, but its implementation is private.

// prmCommon is the common type field in a JSON encoding of PolicyReferenceMatch.
type prmCommon struct {
	Type prmTypeIdentifier `json:"type"`
}

// prmTypeIdentifier is string designating a kind of a PolicyReferenceMatch.
type prmTypeIdentifier string

const (
	prmTypeMatchExact      prmTypeIdentifier = "matchExact"
	prmTypeMatchRepository prmTypeIdentifier = "matchRepository"
	prmTypeExactReference  prmTypeIdentifier = "exactReference"
	prmTypeExactRepository prmTypeIdentifier = "exactRepository"
)

// prmMatchExact is a PolicyReferenceMatch with type = prmMatchExact: the two references must match exactly.
type prmMatchExact struct {
	prmCommon
}

// prmMatchRepository is a PolicyReferenceMatch with type = prmMatchRepository: the two references must use the same repository, may differ in the tag.
type prmMatchRepository struct {
	prmCommon
}

// prmExactReference is a PolicyReferenceMatch with type = prmExactReference: matches a specified reference exactly.
type prmExactReference struct {
	prmCommon
	DockerReference string `json:"dockerReference"`
}

// prmExactRepository is a PolicyReferenceMatch with type = prmExactRepository: matches a specified repository, with any tag.
type prmExactRepository struct {
	prmCommon
	DockerRepository string `json:"dockerRepository"`
}
