package main

import (
	"testing"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterSemVer(t *testing.T) {
	for _, c := range []struct {
		testData     []string
		regex        string
		expectedData []string
	}{
		{
			[]string{
				"docker.io/library/busybox:notlatest",
				"docker.io/library/busybox:latest",
				"docker.io/library/busybox:1.2.3",
				"docker.io/library/busybox:1.3.4",
			},
			"> 1.2",
			[]string{
				"docker.io/library/busybox:1.2.3",
				"docker.io/library/busybox:1.3.4",
			},
		},
		{
			[]string{
				"docker.io/library/busybox:latest",
				"docker.io/library/busybox:1.2.3",
				"docker.io/library/busybox:1.2.6",
				"docker.io/library/busybox:1.3.4",
			},
			"~> 1.2.0",
			[]string{
				"docker.io/library/busybox:1.2.3",
				"docker.io/library/busybox:1.2.6",
			},
		},
	} {

		var testRefs []types.ImageReference
		var expectedRefs []types.ImageReference

		for _, r := range c.testData {
			parsed, _ := reference.ParseNormalizedNamed(r)
			ref, err := docker.NewReference(parsed)
			require.NoError(t, err)
			testRefs = append(testRefs, ref)
		}

		for _, r := range c.expectedData {
			parsed, _ := reference.ParseNormalizedNamed(r)
			ref, err := docker.NewReference(parsed)
			require.NoError(t, err)
			expectedRefs = append(expectedRefs, ref)
		}

		res, err := filterSemVer(c.regex, testRefs)
		require.NoError(t, err)
		assert.Equal(t, expectedRefs, res, res)
	}
}

func TestFilterRegex(t *testing.T) {
	var testRefs []types.ImageReference
	var expectedRefs []types.ImageReference

	for _, c := range []struct {
		testData     []string
		regex        string
		expectedData []string
	}{
		{
			[]string{
				"docker.io/library/busybox:notlatest",
				"docker.io/library/busybox:latest",
				"docker.io/library/busybox:1.2.3",
				"docker.io/library/busybox:1.3.4",
			},
			"^1.2.*",
			[]string{
				"docker.io/library/busybox:1.2.3",
			},
		},
		{
			[]string{
				"docker.io/library/busybox:latest",
				"docker.io/library/busybox:1.3.4",
			},
			"^1.2.*",
			[]string{},
		},
	} {

		for _, r := range c.testData {
			parsed, _ := reference.ParseNormalizedNamed(r)
			ref, err := docker.NewReference(parsed)
			require.NoError(t, err)
			testRefs = append(testRefs, ref)
		}

		for _, r := range c.expectedData {
			parsed, _ := reference.ParseNormalizedNamed(r)
			ref, err := docker.NewReference(parsed)
			require.NoError(t, err)
			expectedRefs = append(expectedRefs, ref)
		}

		res, err := filterRegex(c.regex, testRefs)
		require.NoError(t, err)
		assert.Equal(t, expectedRefs, res, res)
	}
}
