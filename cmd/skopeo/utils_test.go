package main

import (
	"flag"
	"os"
	"testing"

	"github.com/containers/image/v5/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGlobalOptions creates globalOptions and sets it according to flags.
// NOTE: This is QUITE FAKE; none of the urfave/cli normalization and the like happens.
func fakeGlobalOptions(t *testing.T, flags []string) *globalOptions {
	app, opts := createApp()

	flagSet := flag.NewFlagSet(app.Name, flag.ContinueOnError)
	for _, f := range app.Flags {
		f.Apply(flagSet)
	}
	err := flagSet.Parse(flags)
	require.NoError(t, err)

	return opts
}

// fakeImageOptions creates imageOptions and sets it according to globalFlags/cmdFlags.
// NOTE: This is QUITE FAKE; none of the urfave/cli normalization and the like happens.
func fakeImageOptions(t *testing.T, flagPrefix string, globalFlags []string, cmdFlags []string) *imageOptions {
	globalOpts := fakeGlobalOptions(t, globalFlags)

	sharedFlags, sharedOpts := sharedImageFlags()
	imageFlags, imageOpts := imageFlags(globalOpts, sharedOpts, flagPrefix, "")
	flagSet := flag.NewFlagSet("fakeImageOptions", flag.ContinueOnError)
	for _, f := range append(sharedFlags, imageFlags...) {
		f.Apply(flagSet)
	}
	err := flagSet.Parse(cmdFlags)
	require.NoError(t, err)
	return imageOpts
}

func TestImageOptionsNewSystemContext(t *testing.T) {
	// Default state
	opts := fakeImageOptions(t, "dest-", []string{}, []string{})
	res, err := opts.newSystemContext()
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{}, res)

	// Set everything to non-default values.
	opts = fakeImageOptions(t, "dest-", []string{
		"--registries.d", "/srv/registries.d",
		"--override-arch", "overridden-arch",
		"--override-os", "overridden-os",
		"--override-variant", "overridden-variant",
	}, []string{
		"--authfile", "/srv/authfile",
		"--dest-authfile", "/srv/dest-authfile",
		"--dest-cert-dir", "/srv/cert-dir",
		"--dest-shared-blob-dir", "/srv/shared-blob-dir",
		"--dest-daemon-host", "daemon-host.example.com",
		"--dest-tls-verify=false",
		"--dest-creds", "creds-user:creds-password",
	})
	res, err = opts.newSystemContext()
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{
		RegistriesDirPath:                 "/srv/registries.d",
		AuthFilePath:                      "/srv/dest-authfile",
		ArchitectureChoice:                "overridden-arch",
		OSChoice:                          "overridden-os",
		VariantChoice:                     "overridden-variant",
		OCISharedBlobDirPath:              "/srv/shared-blob-dir",
		DockerCertPath:                    "/srv/cert-dir",
		DockerInsecureSkipTLSVerify:       types.OptionalBoolTrue,
		DockerAuthConfig:                  &types.DockerAuthConfig{Username: "creds-user", Password: "creds-password"},
		DockerDaemonCertPath:              "/srv/cert-dir",
		DockerDaemonHost:                  "daemon-host.example.com",
		DockerDaemonInsecureSkipTLSVerify: true,
	}, res)

	// Global/per-command tlsVerify behavior
	for _, c := range []struct {
		global, cmd          string
		expectedDocker       types.OptionalBool
		expectedDockerDaemon bool
	}{
		{"", "", types.OptionalBoolUndefined, false},
		{"", "false", types.OptionalBoolTrue, true},
		{"", "true", types.OptionalBoolFalse, false},
		{"false", "", types.OptionalBoolTrue, false},
		{"false", "false", types.OptionalBoolTrue, true},
		{"false", "true", types.OptionalBoolFalse, false},
		{"true", "", types.OptionalBoolFalse, false},
		{"true", "false", types.OptionalBoolTrue, true},
		{"true", "true", types.OptionalBoolFalse, false},
	} {
		globalFlags := []string{}
		if c.global != "" {
			globalFlags = append(globalFlags, "--tls-verify="+c.global)
		}
		cmdFlags := []string{}
		if c.cmd != "" {
			cmdFlags = append(cmdFlags, "--dest-tls-verify="+c.cmd)
		}
		opts := fakeImageOptions(t, "dest-", globalFlags, cmdFlags)
		res, err = opts.newSystemContext()
		require.NoError(t, err)
		assert.Equal(t, c.expectedDocker, res.DockerInsecureSkipTLSVerify, "%#v", c)
		assert.Equal(t, c.expectedDockerDaemon, res.DockerDaemonInsecureSkipTLSVerify, "%#v", c)
	}

	// Invalid option values
	opts = fakeImageOptions(t, "dest-", []string{}, []string{"--dest-creds", ""})
	_, err = opts.newSystemContext()
	assert.Error(t, err)
}

// fakeImageDestOptions creates imageDestOptions and sets it according to globalFlags/cmdFlags.
// NOTE: This is QUITE FAKE; none of the urfave/cli normalization and the like happens.
func fakeImageDestOptions(t *testing.T, flagPrefix string, globalFlags []string, cmdFlags []string) *imageDestOptions {
	globalOpts := fakeGlobalOptions(t, globalFlags)

	sharedFlags, sharedOpts := sharedImageFlags()
	imageFlags, imageOpts := imageDestFlags(globalOpts, sharedOpts, flagPrefix, "")
	flagSet := flag.NewFlagSet("fakeImageDestOptions", flag.ContinueOnError)
	for _, f := range append(sharedFlags, imageFlags...) {
		f.Apply(flagSet)
	}
	err := flagSet.Parse(cmdFlags)
	require.NoError(t, err)
	return imageOpts
}

func TestImageDestOptionsNewSystemContext(t *testing.T) {
	// Default state
	opts := fakeImageDestOptions(t, "dest-", []string{}, []string{})
	res, err := opts.newSystemContext()
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{}, res)

	oldXRD, hasXRD := os.LookupEnv("REGISTRY_AUTH_FILE")
	defer func() {
		if hasXRD {
			os.Setenv("REGISTRY_AUTH_FILE", oldXRD)
		} else {
			os.Unsetenv("REGISTRY_AUTH_FILE")
		}
	}()
	authFile := "/tmp/auth.json"
	// Make sure when REGISTRY_AUTH_FILE is set the auth file is used
	os.Setenv("REGISTRY_AUTH_FILE", authFile)

	// Explicitly set everything to default, except for when the default is “not present”
	opts = fakeImageDestOptions(t, "dest-", []string{}, []string{
		"--dest-compress=false",
	})
	res, err = opts.newSystemContext()
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{AuthFilePath: authFile}, res)

	// Set everything to non-default values.
	opts = fakeImageDestOptions(t, "dest-", []string{
		"--registries.d", "/srv/registries.d",
		"--override-arch", "overridden-arch",
		"--override-os", "overridden-os",
		"--override-variant", "overridden-variant",
	}, []string{
		"--authfile", "/srv/authfile",
		"--dest-cert-dir", "/srv/cert-dir",
		"--dest-shared-blob-dir", "/srv/shared-blob-dir",
		"--dest-compress=true",
		"--dest-daemon-host", "daemon-host.example.com",
		"--dest-tls-verify=false",
		"--dest-creds", "creds-user:creds-password",
	})
	res, err = opts.newSystemContext()
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{
		RegistriesDirPath:                 "/srv/registries.d",
		AuthFilePath:                      "/srv/authfile",
		ArchitectureChoice:                "overridden-arch",
		OSChoice:                          "overridden-os",
		VariantChoice:                     "overridden-variant",
		OCISharedBlobDirPath:              "/srv/shared-blob-dir",
		DockerCertPath:                    "/srv/cert-dir",
		DockerInsecureSkipTLSVerify:       types.OptionalBoolTrue,
		DockerAuthConfig:                  &types.DockerAuthConfig{Username: "creds-user", Password: "creds-password"},
		DockerDaemonCertPath:              "/srv/cert-dir",
		DockerDaemonHost:                  "daemon-host.example.com",
		DockerDaemonInsecureSkipTLSVerify: true,
		DirForceCompress:                  true,
	}, res)

	// Invalid option values in imageOptions
	opts = fakeImageDestOptions(t, "dest-", []string{}, []string{"--dest-creds", ""})
	_, err = opts.newSystemContext()
	assert.Error(t, err)
}

// since there is a shared authfile image option and a non-shared (prefixed) one, make sure the override logic
// works correctly.
func TestImageOptionsAuthfileOverride(t *testing.T) {

	for _, testCase := range []struct {
		flagPrefix           string
		cmdFlags             []string
		expectedAuthfilePath string
	}{
		// if there is no prefix, only authfile is allowed.
		{"",
			[]string{
				"--authfile", "/srv/authfile",
			}, "/srv/authfile"},
		// if authfile and dest-authfile is provided, dest-authfile wins
		{"dest-",
			[]string{
				"--authfile", "/srv/authfile",
				"--dest-authfile", "/srv/dest-authfile",
			}, "/srv/dest-authfile",
		},
		// if only the shared authfile is provided, authfile must be present in system context
		{"dest-",
			[]string{
				"--authfile", "/srv/authfile",
			}, "/srv/authfile",
		},
		// if only the dest authfile is provided, dest-authfile must be present in system context
		{"dest-",
			[]string{
				"--dest-authfile", "/srv/dest-authfile",
			}, "/srv/dest-authfile",
		},
	} {
		opts := fakeImageOptions(t, testCase.flagPrefix, []string{}, testCase.cmdFlags)
		res, err := opts.newSystemContext()
		require.NoError(t, err)

		assert.Equal(t, &types.SystemContext{
			AuthFilePath: testCase.expectedAuthfilePath,
		}, res)
	}
}
