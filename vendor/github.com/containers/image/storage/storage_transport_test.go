package storage

import (
	"fmt"
	"testing"

	"github.com/containers/image/docker/reference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sha256digestHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func TestTransportName(t *testing.T) {
	assert.Equal(t, "containers-storage", Transport.Name())
}

func TestTransportParseStoreReference(t *testing.T) {
	for _, c := range []struct{ input, expectedRef, expectedID string }{
		{"", "", ""}, // Empty input
		// Handling of the store prefix
		// FIXME? Should we be silently discarding input like this?
		{"[unterminated", "", ""},                                    // Unterminated store specifier
		{"[garbage]busybox", "docker.io/library/busybox:latest", ""}, // Store specifier is overridden by the store we pass to ParseStoreReference

		{"UPPERCASEISINVALID", "", ""},                                                     // Invalid single-component name
		{"sha256:" + sha256digestHex, "docker.io/library/sha256:" + sha256digestHex, ""},   // Valid single-component name; the hex part is not an ID unless it has a "@" prefix
		{sha256digestHex, "", ""},                                                          // Invalid single-component ID; not an ID without a "@" prefix, so it's parsed as a name, but names aren't allowed to look like IDs
		{"@" + sha256digestHex, "", sha256digestHex},                                       // Valid single-component ID
		{"sha256:ab", "docker.io/library/sha256:ab", ""},                                   // Valid single-component name, explicit tag
		{"busybox", "docker.io/library/busybox:latest", ""},                                // Valid single-component name, implicit tag
		{"busybox:notlatest", "docker.io/library/busybox:notlatest", ""},                   // Valid single-component name, explicit tag
		{"docker.io/library/busybox:notlatest", "docker.io/library/busybox:notlatest", ""}, // Valid single-component name, everything explicit

		{"UPPERCASEISINVALID@" + sha256digestHex, "", ""},                                                                  // Invalid name in name@ID
		{"busybox@ab", "", ""},                                                                                             // Invalid ID in name@ID
		{"busybox@", "", ""},                                                                                               // Empty ID in name@ID
		{"busybox@sha256:" + sha256digestHex, "", ""},                                                                      // This (a digested docker/docker reference format) is also invalid, since it's an invalid ID in name@ID
		{"@" + sha256digestHex, "", sha256digestHex},                                                                       // Valid two-component name, with ID only
		{"busybox@" + sha256digestHex, "docker.io/library/busybox:latest", sha256digestHex},                                // Valid two-component name, implicit tag
		{"busybox:notlatest@" + sha256digestHex, "docker.io/library/busybox:notlatest", sha256digestHex},                   // Valid two-component name, explicit tag
		{"docker.io/library/busybox:notlatest@" + sha256digestHex, "docker.io/library/busybox:notlatest", sha256digestHex}, // Valid two-component name, everything explicit
	} {
		storageRef, err := Transport.ParseStoreReference(Transport.(*storageTransport).store, c.input)
		if c.expectedRef == "" && c.expectedID == "" {
			assert.Error(t, err, c.input)
		} else {
			require.NoError(t, err, c.input)
			assert.Equal(t, *(Transport.(*storageTransport)), storageRef.transport, c.input)
			assert.Equal(t, c.expectedRef, storageRef.reference, c.input)
			assert.Equal(t, c.expectedID, storageRef.id, c.input)
			if c.expectedRef == "" {
				assert.Nil(t, storageRef.name, c.input)
			} else {
				dockerRef, err := reference.ParseNormalizedNamed(c.expectedRef)
				require.NoError(t, err)
				require.NotNil(t, storageRef.name, c.input)
				assert.Equal(t, dockerRef.String(), storageRef.name.String())
			}
		}
	}
}

func TestTransportParseReference(t *testing.T) {
	store := newStore(t)
	driver := store.GetGraphDriverName()
	root := store.GetGraphRoot()

	for _, c := range []struct{ prefix, expectedDriver, expectedRoot string }{
		{"", driver, root},                              // Implicit store location prefix
		{"[unterminated", "", ""},                       // Unterminated store specifier
		{"[]", "", ""},                                  // Empty store specifier
		{"[relative/path]", "", ""},                     // Non-absolute graph root path
		{"[" + driver + "@relative/path]", "", ""},      // Non-absolute graph root path
		{"[thisisunknown@" + root + "suffix2]", "", ""}, // Unknown graph driver

		// The next two could be valid, but aren't enough to allow GetStore() to locate a matching
		// store, since the reference can't specify a RunRoot.  Without one, GetStore() tries to
		// match the GraphRoot (possibly combined with the driver name) against a Store that was
		// previously opened using GetStore(), and we haven't done that.
		// Future versions of the storage library will probably make this easier for locations that
		// are shared, by caching the rest of the information inside the graph root so that it can
		// be looked up later, but since this is a per-test temporary location, that won't help here.
		//{"[" + root + "suffix1]", driver, root + "suffix1"},                // A valid root path
		//{"[" + driver + "@" + root + "suffix3]", driver, root + "suffix3"}, // A valid root@graph pair
	} {
		ref, err := Transport.ParseReference(c.prefix + "busybox")
		if c.expectedDriver == "" {
			assert.Error(t, err, c.prefix)
		} else {
			require.NoError(t, err, c.prefix)
			storageRef, ok := ref.(*storageReference)
			require.True(t, ok, c.prefix)
			assert.Equal(t, c.expectedDriver, storageRef.transport.store.GetGraphDriverName(), c.prefix)
			assert.Equal(t, c.expectedRoot, storageRef.transport.store.GetGraphRoot(), c.prefix)
		}
	}
}

func TestTransportValidatePolicyConfigurationScope(t *testing.T) {
	store := newStore(t)
	driver := store.GetGraphDriverName()
	root := store.GetGraphRoot()
	storeSpec := fmt.Sprintf("[%s@%s]", driver, root) // As computed in PolicyConfigurationNamespaces

	// Valid inputs
	for _, scope := range []string{
		"[" + root + "suffix1]",                                              // driverlessStoreSpec in PolicyConfigurationNamespaces
		"[" + driver + "@" + root + "suffix3]",                               // storeSpec in PolicyConfigurationNamespaces
		storeSpec + "sha256:ab",                                              // Valid single-component name, explicit tag
		storeSpec + "sha256:" + sha256digestHex,                              // Valid single-component ID with a longer explicit tag
		storeSpec + "busybox",                                                // Valid single-component name, implicit tag; NOTE that this non-canonical form would be interpreted as a scope for host busybox
		storeSpec + "busybox:notlatest",                                      // Valid single-component name, explicit tag; NOTE that this non-canonical form would be interpreted as a scope for host busybox
		storeSpec + "docker.io/library/busybox:notlatest",                    // Valid single-component name, everything explicit
		storeSpec + "busybox@" + sha256digestHex,                             // Valid two-component name, implicit tag; NOTE that this non-canonical form would be interpreted as a scope for host busybox (and never match)
		storeSpec + "busybox:notlatest@" + sha256digestHex,                   // Valid two-component name, explicit tag; NOTE that this non-canonical form would be interpreted as a scope for host busybox (and never match)
		storeSpec + "docker.io/library/busybox:notlatest@" + sha256digestHex, // Valid two-component name, everything explicit
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.NoError(t, err, scope)
	}

	// Invalid inputs
	for _, scope := range []string{
		"busybox",                        // Unprefixed reference
		"[unterminated",                  // Unterminated store specifier
		"[]",                             // Empty store specifier
		"[relative/path]",                // Non-absolute graph root path
		"[" + driver + "@relative/path]", // Non-absolute graph root path
		// "[thisisunknown@" + root + "suffix2]", // Unknown graph driver FIXME: validate against storage.ListGraphDrivers() once that's available
		storeSpec + sha256digestHex,       // Almost a valid single-component name, but rejected because it looks like an ID that's missing its "@" prefix
		storeSpec + "@",                   // An incomplete two-component name
		storeSpec + "@" + sha256digestHex, // A valid two-component name, but ID-only, so not a valid scope

		storeSpec + "UPPERCASEISINVALID",                    // Invalid single-component name
		storeSpec + "UPPERCASEISINVALID@" + sha256digestHex, // Invalid name in name@ID
		storeSpec + "busybox@ab",                            // Invalid ID in name@ID
		storeSpec + "busybox@",                              // Empty ID in name@ID
		storeSpec + "busybox@sha256:" + sha256digestHex,     // This (in a digested docker/docker reference format) is also invalid; this can't actually be matched by a storageReference.PolicyConfigurationIdentity, so it should be rejected
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.Error(t, err, scope)
	}
}
