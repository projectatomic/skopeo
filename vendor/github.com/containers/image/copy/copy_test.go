package copy

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/containers/image/pkg/compression"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDigestingReader(t *testing.T) {
	// Only the failure cases, success is tested in TestDigestingReaderRead below.
	source := bytes.NewReader([]byte("abc"))
	for _, input := range []digest.Digest{
		"abc",             // Not algo:hexvalue
		"crc32:",          // Unknown algorithm, empty value
		"crc32:012345678", // Unknown algorithm
		"sha256:",         // Empty value
		"sha256:0",        // Invalid hex value
		"sha256:01",       // Invalid length of hex value
	} {
		_, err := newDigestingReader(source, input)
		assert.Error(t, err, input.String())
	}
}

func TestDigestingReaderRead(t *testing.T) {
	cases := []struct {
		input  []byte
		digest digest.Digest
	}{
		{[]byte(""), "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{[]byte("abc"), "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{make([]byte, 65537, 65537), "sha256:3266304f31be278d06c3bd3eb9aa3e00c59bedec0a890de466568b0b90b0e01f"},
	}
	// Valid input
	for _, c := range cases {
		source := bytes.NewReader(c.input)
		reader, err := newDigestingReader(source, c.digest)
		require.NoError(t, err, c.digest.String())
		dest := bytes.Buffer{}
		n, err := io.Copy(&dest, reader)
		assert.NoError(t, err, c.digest.String())
		assert.Equal(t, int64(len(c.input)), n, c.digest.String())
		assert.Equal(t, c.input, dest.Bytes(), c.digest.String())
		assert.False(t, reader.validationFailed, c.digest.String())
	}
	// Modified input
	for _, c := range cases {
		source := bytes.NewReader(bytes.Join([][]byte{c.input, []byte("x")}, nil))
		reader, err := newDigestingReader(source, c.digest)
		require.NoError(t, err, c.digest.String())
		dest := bytes.Buffer{}
		_, err = io.Copy(&dest, reader)
		assert.Error(t, err, c.digest.String())
		assert.True(t, reader.validationFailed)
	}
}

func goDiffIDComputationGoroutineWithTimeout(layerStream io.ReadCloser, decompressor compression.DecompressorFunc) *diffIDResult {
	ch := make(chan diffIDResult)
	go diffIDComputationGoroutine(ch, layerStream, nil)
	timeout := time.After(time.Second)
	select {
	case res := <-ch:
		return &res
	case <-timeout:
		return nil
	}
}

func TestDiffIDComputationGoroutine(t *testing.T) {
	stream, err := os.Open("fixtures/Hello.uncompressed")
	require.NoError(t, err)
	res := goDiffIDComputationGoroutineWithTimeout(stream, nil)
	require.NotNil(t, res)
	assert.NoError(t, res.err)
	assert.Equal(t, "sha256:185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969", res.digest.String())

	// Error reading input
	reader, writer := io.Pipe()
	writer.CloseWithError(errors.New("Expected error reading input in diffIDComputationGoroutine"))
	res = goDiffIDComputationGoroutineWithTimeout(reader, nil)
	require.NotNil(t, res)
	assert.Error(t, res.err)
}

func TestComputeDiffID(t *testing.T) {
	for _, c := range []struct {
		filename     string
		decompressor compression.DecompressorFunc
		result       digest.Digest
	}{
		{"fixtures/Hello.uncompressed", nil, "sha256:185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969"},
		{"fixtures/Hello.gz", nil, "sha256:0bd4409dcd76476a263b8f3221b4ce04eb4686dec40bfdcc2e86a7403de13609"},
		{"fixtures/Hello.gz", compression.GzipDecompressor, "sha256:185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969"},
	} {
		stream, err := os.Open(c.filename)
		require.NoError(t, err, c.filename)
		defer stream.Close()

		diffID, err := computeDiffID(stream, c.decompressor)
		require.NoError(t, err, c.filename)
		assert.Equal(t, c.result, diffID)
	}

	// Error initializing decompression
	_, err := computeDiffID(bytes.NewReader([]byte{}), compression.GzipDecompressor)
	assert.Error(t, err)

	// Error reading input
	reader, writer := io.Pipe()
	defer reader.Close()
	writer.CloseWithError(errors.New("Expected error reading input in computeDiffID"))
	_, err = computeDiffID(reader, nil)
	assert.Error(t, err)
}
