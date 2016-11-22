package daemon

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
)

type daemonImageDestination struct {
	ref            daemonReference
	namedTaggedRef reference.NamedTagged // Strictly speaking redundant with ref above; having the field makes it structurally impossible for later users to fail.
	// For talking to imageLoadGoroutine
	goroutineCancel context.CancelFunc
	statusChannel   <-chan error
	writer          *io.PipeWriter
	tar             *tar.Writer
	// Other state
	committed bool // writer has been closed
}

// newImageDestination returns a types.ImageDestination for the specified image reference.
func newImageDestination(systemCtx *types.SystemContext, ref daemonReference) (types.ImageDestination, error) {
	if ref.ref == nil {
		return nil, fmt.Errorf("Invalid destination docker-daemon:%s: a destination must be a name:tag", ref.StringWithinTransport())
	}
	namedTaggedRef, ok := ref.ref.(reference.NamedTagged)
	if !ok {
		return nil, fmt.Errorf("Invalid destination docker-daemon:%s: a destination must be a name:tag", ref.StringWithinTransport())
	}

	c, err := client.NewClient(client.DefaultDockerHost, "1.22", nil, nil) // FIXME: overridable host
	if err != nil {
		return nil, fmt.Errorf("Error initializing docker engine client: %v", err)
	}

	reader, writer := io.Pipe()
	// Commit() may never be called, so we may never read from this channel; so, make this buffered to allow imageLoadGoroutine to write status and terminate even if we never read it.
	statusChannel := make(chan error, 1)

	ctx, goroutineCancel := context.WithCancel(context.Background())
	go imageLoadGoroutine(ctx, c, reader, statusChannel)

	return &daemonImageDestination{
		ref:             ref,
		namedTaggedRef:  namedTaggedRef,
		goroutineCancel: goroutineCancel,
		statusChannel:   statusChannel,
		writer:          writer,
		tar:             tar.NewWriter(writer),
		committed:       false,
	}, nil
}

// imageLoadGoroutine accepts tar stream on reader, sends it to c, and reports error or success by writing to statusChannel
func imageLoadGoroutine(ctx context.Context, c *client.Client, reader *io.PipeReader, statusChannel chan<- error) {
	err := errors.New("Internal error: unexpected panic in imageLoadGoroutine")
	defer func() {
		logrus.Debugf("docker-daemon: sending done, status %v", err)
		statusChannel <- err
	}()
	defer func() {
		if err == nil {
			reader.Close()
		} else {
			reader.CloseWithError(err)
		}
	}()

	resp, err := c.ImageLoad(ctx, reader, true)
	if err != nil {
		err = fmt.Errorf("Error saving image to docker engine: %v", err)
		return
	}
	defer resp.Body.Close()
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *daemonImageDestination) Close() {
	if !d.committed {
		logrus.Debugf("docker-daemon: Closing tar stream to abort loading")
		// In principle, goroutineCancel() should abort the HTTP request and stop the process from continuing.
		// In practice, though, https://github.com/docker/engine-api/blob/master/client/transport/cancellable/cancellable.go
		// currently just runs the HTTP request to completion in a goroutine, and returns early if the context is canceled
		// without terminating the HTTP request at all.  So we need this CloseWithError to terminate sending the HTTP request Body
		// immediately, and hopefully, through terminating the sending which uses "Transfer-Encoding: chunked"" without sending
		// the terminating zero-length chunk, prevent the docker daemon from processing the tar stream at all.
		// Whether that works or not, closing the PipeWriter seems desirable in any case.
		d.writer.CloseWithError(errors.New("Aborting upload, daemonImageDestination closed without a previous .Commit()"))
	}
	d.goroutineCancel()
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *daemonImageDestination) Reference() types.ImageReference {
	return d.ref
}

// SupportedManifestMIMETypes tells which manifest mime types the destination supports
// If an empty slice or nil it's returned, then any mime type can be tried to upload
func (d *daemonImageDestination) SupportedManifestMIMETypes() []string {
	return []string{
		manifest.DockerV2Schema2MediaType, // We rely on the types.Image.UpdatedImage schema conversion capabilities.
	}
}

// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures.
// Note: It is still possible for PutSignatures to fail if SupportsSignatures returns nil.
func (d *daemonImageDestination) SupportsSignatures() error {
	return fmt.Errorf("Storing signatures for docker-daemon: destinations is not supported")
}

// ShouldCompressLayers returns true iff it is desirable to compress layer blobs written to this destination.
func (d *daemonImageDestination) ShouldCompressLayers() bool {
	return false
}

// PutBlob writes contents of stream and returns data representing the result (with all data filled in).
// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
// inputInfo.Size is the expected length of stream, if known.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *daemonImageDestination) PutBlob(stream io.Reader, inputInfo types.BlobInfo) (types.BlobInfo, error) {
	if inputInfo.Digest == "" {
		return types.BlobInfo{}, fmt.Errorf(`"Can not stream a blob with unknown digest to "docker-daemon:"`)
	}

	if inputInfo.Size == -1 { // Ouch, we need to stream the blob into a temporary file just to determine the size.
		logrus.Debugf("docker-daemon: input with unknown size, streaming to disk first…")
		streamCopy, err := ioutil.TempFile(temporaryDirectoryForBigFiles, "docker-daemon-blob")
		if err != nil {
			return types.BlobInfo{}, err
		}
		defer os.Remove(streamCopy.Name())
		defer streamCopy.Close()

		size, err := io.Copy(streamCopy, stream)
		if err != nil {
			return types.BlobInfo{}, err
		}
		_, err = streamCopy.Seek(0, os.SEEK_SET)
		if err != nil {
			return types.BlobInfo{}, err
		}
		inputInfo.Size = size // inputInfo is a struct, so we are only modifying our copy.
		stream = streamCopy
		logrus.Debugf("… streaming done")
	}

	hash := sha256.New()
	tee := io.TeeReader(stream, hash)
	if err := d.sendFile(inputInfo.Digest, inputInfo.Size, tee); err != nil {
		return types.BlobInfo{}, err
	}
	return types.BlobInfo{Digest: "sha256:" + hex.EncodeToString(hash.Sum(nil)), Size: inputInfo.Size}, nil
}

func (d *daemonImageDestination) PutManifest(m []byte) error {
	var man schema2Manifest
	if err := json.Unmarshal(m, &man); err != nil {
		return fmt.Errorf("Error parsing manifest: %v", err)
	}
	if man.SchemaVersion != 2 || man.MediaType != manifest.DockerV2Schema2MediaType {
		return fmt.Errorf("Unsupported manifest type, need a Docker schema 2 manifest")
	}

	layerPaths := []string{}
	for _, l := range man.Layers {
		layerPaths = append(layerPaths, l.Digest)
	}

	// For github.com/docker/docker consumers, this works just as well as
	//   refString := d.namedTaggedRef.String()  [i.e. d.ref.ref.String()]
	// because when reading the RepoTags strings, github.com/docker/docker/reference
	// normalizes both of them to the same value.
	//
	// Doing it this way to include the normalized-out `docker.io[/library]` does make
	// a difference for github.com/projectatomic/docker consumers, with the
	// “Add --add-registry and --block-registry options to docker daemon” patch.
	// These consumers treat reference strings which include a hostname and reference
	// strings without a hostname differently.
	//
	// Using the host name here is more explicit about the intent, and it has the same
	// effect as (docker pull) in projectatomic/docker, which tags the result using
	// a hostname-qualified reference.
	// See https://github.com/containers/image/issues/72 for a more detailed
	// analysis and explanation.
	refString := fmt.Sprintf("%s:%s", d.namedTaggedRef.FullName(), d.namedTaggedRef.Tag())

	items := []manifestItem{{
		Config:       man.Config.Digest,
		RepoTags:     []string{refString},
		Layers:       layerPaths,
		Parent:       "",
		LayerSources: nil,
	}}
	itemsBytes, err := json.Marshal(&items)
	if err != nil {
		return err
	}

	// FIXME? Do we also need to support the legacy format?
	return d.sendFile(manifestFileName, int64(len(itemsBytes)), bytes.NewReader(itemsBytes))
}

type tarFI struct {
	path string
	size int64
}

func (t *tarFI) Name() string {
	return t.path
}
func (t *tarFI) Size() int64 {
	return t.size
}
func (t *tarFI) Mode() os.FileMode {
	return 0444
}
func (t *tarFI) ModTime() time.Time {
	return time.Unix(0, 0)
}
func (t *tarFI) IsDir() bool {
	return false
}
func (t *tarFI) Sys() interface{} {
	return nil
}

// sendFile sends a file into the tar stream.
func (d *daemonImageDestination) sendFile(path string, expectedSize int64, stream io.Reader) error {
	hdr, err := tar.FileInfoHeader(&tarFI{path: path, size: expectedSize}, "")
	if err != nil {
		return nil
	}
	logrus.Debugf("Sending as tar file %s", path)
	if err := d.tar.WriteHeader(hdr); err != nil {
		return err
	}
	size, err := io.Copy(d.tar, stream)
	if err != nil {
		return err
	}
	if size != expectedSize {
		return fmt.Errorf("Size mismatch when copying %s, expected %d, got %d", path, expectedSize, size)
	}
	return nil
}

func (d *daemonImageDestination) PutSignatures(signatures [][]byte) error {
	if len(signatures) != 0 {
		return fmt.Errorf("Storing signatures for docker-daemon: destinations is not supported")
	}
	return nil
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *daemonImageDestination) Commit() error {
	logrus.Debugf("docker-daemon: Closing tar stream")
	if err := d.tar.Close(); err != nil {
		return err
	}
	if err := d.writer.Close(); err != nil {
		return err
	}
	d.committed = true // We may still fail, but we are done sending to imageLoadGoroutine.

	logrus.Debugf("docker-daemon: Waiting for status")
	err := <-d.statusChannel
	return err
}
