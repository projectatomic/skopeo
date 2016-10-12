package copy

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"io"

	"github.com/Sirupsen/logrus"
)

// decompressorFunc, given a compressed stream, returns the decompressed stream.
type decompressorFunc func(io.Reader) (io.Reader, error)

func gzipDecompressor(r io.Reader) (io.Reader, error) {
	return gzip.NewReader(r)
}
func bzip2Decompressor(r io.Reader) (io.Reader, error) {
	return bzip2.NewReader(r), nil
}
func xzDecompressor(r io.Reader) (io.Reader, error) {
	return nil, errors.New("Decompressing xz streams is not supported")
}

// compressionAlgos is an internal implementation detail of detectCompression
var compressionAlgos = map[string]struct {
	prefix       []byte
	decompressor decompressorFunc
}{
	"gzip":  {[]byte{0x1F, 0x8B, 0x08}, gzipDecompressor},                 // gzip (RFC 1952)
	"bzip2": {[]byte{0x42, 0x5A, 0x68}, bzip2Decompressor},                // bzip2 (decompress.c:BZ2_decompress)
	"xz":    {[]byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00}, xzDecompressor}, // xz (/usr/share/doc/xz/xz-file-format.txt)
}

// detectCompression returns a decompressorFunc if the input is recognized as a compressed format, nil otherwise.
// Because it consumes the start of input, other consumers must use the returned io.Reader instead to also read from the beginning.
func detectCompression(input io.Reader) (decompressorFunc, io.Reader, error) {
	buffer := [8]byte{}

	n, err := io.ReadAtLeast(input, buffer[:], len(buffer))
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		// This is a “real” error. We could just ignore it this time, process the data we have, and hope that the source will report the same error again.
		// Instead, fail immediately with the original error cause instead of a possibly secondary/misleading error returned later.
		return nil, nil, err
	}

	var decompressor decompressorFunc
	for name, algo := range compressionAlgos {
		if bytes.HasPrefix(buffer[:n], algo.prefix) {
			logrus.Debugf("Detected compression format %s", name)
			decompressor = algo.decompressor
			break
		}
	}
	if decompressor == nil {
		logrus.Debugf("No compression detected")
	}

	return decompressor, io.MultiReader(bytes.NewReader(buffer[:n]), input), nil
}
