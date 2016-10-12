package copy

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"reflect"
	"strings"

	pb "gopkg.in/cheggaaa/pb.v1"

	"github.com/Sirupsen/logrus"
	"github.com/containers/image/image"
	"github.com/containers/image/manifest"
	"github.com/containers/image/signature"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
)

// preferredManifestMIMETypes lists manifest MIME types in order of our preference, if we can't use the original manifest and need to convert.
// Prefer v2s2 to v2s1 because v2s2 does not need to be changed when uploading to a different location.
// Include v2s1 signed but not v2s1 unsigned, because docker/distribution requires a signature even if the unsigned MIME type is used.
var preferredManifestMIMETypes = []string{manifest.DockerV2Schema2MediaType, manifest.DockerV2Schema1SignedMediaType}

// supportedDigests lists the supported blob digest types.
var supportedDigests = map[string]func() hash.Hash{
	"sha256": sha256.New,
}

type digestingReader struct {
	source           io.Reader
	digest           hash.Hash
	expectedDigest   []byte
	validationFailed bool
}

// newDigestingReader returns an io.Reader implementation with contents of source, which will eventually return a non-EOF error
// and set validationFailed to true if the source stream does not match expectedDigestString.
func newDigestingReader(source io.Reader, expectedDigestString string) (*digestingReader, error) {
	fields := strings.SplitN(expectedDigestString, ":", 2)
	if len(fields) != 2 {
		return nil, fmt.Errorf("Invalid digest specification %s", expectedDigestString)
	}
	fn, ok := supportedDigests[fields[0]]
	if !ok {
		return nil, fmt.Errorf("Invalid digest specification %s: unknown digest type %s", expectedDigestString, fields[0])
	}
	digest := fn()
	expectedDigest, err := hex.DecodeString(fields[1])
	if err != nil {
		return nil, fmt.Errorf("Invalid digest value %s: %v", expectedDigestString, err)
	}
	if len(expectedDigest) != digest.Size() {
		return nil, fmt.Errorf("Invalid digest specification %s: length %d does not match %d", expectedDigestString, len(expectedDigest), digest.Size())
	}
	return &digestingReader{
		source:           source,
		digest:           digest,
		expectedDigest:   expectedDigest,
		validationFailed: false,
	}, nil
}

func (d *digestingReader) Read(p []byte) (int, error) {
	n, err := d.source.Read(p)
	if n > 0 {
		if n2, err := d.digest.Write(p[:n]); n2 != n || err != nil {
			// Coverage: This should not happen, the hash.Hash interface requires
			// d.digest.Write to never return an error, and the io.Writer interface
			// requires n2 == len(input) if no error is returned.
			return 0, fmt.Errorf("Error updating digest during verification: %d vs. %d, %v", n2, n, err)
		}
	}
	if err == io.EOF {
		actualDigest := d.digest.Sum(nil)
		if subtle.ConstantTimeCompare(actualDigest, d.expectedDigest) != 1 {
			d.validationFailed = true
			return 0, fmt.Errorf("Digest did not match, expected %s, got %s", hex.EncodeToString(d.expectedDigest), hex.EncodeToString(actualDigest))
		}
	}
	return n, err
}

// Options allows supplying non-default configuration modifying the behavior of CopyImage.
type Options struct {
	RemoveSignatures bool   // Remove any pre-existing signatures. SignBy will still add a new signature.
	SignBy           string // If non-empty, asks for a signature to be added during the copy, and specifies a key ID, as accepted by signature.NewGPGSigningMechanism().SignDockerManifest(),
	ReportWriter     io.Writer
}

// Image copies image from srcRef to destRef, using policyContext to validate source image admissibility.
func Image(ctx *types.SystemContext, policyContext *signature.PolicyContext, destRef, srcRef types.ImageReference, options *Options) error {
	reportWriter := ioutil.Discard
	if options != nil && options.ReportWriter != nil {
		reportWriter = options.ReportWriter
	}
	writeReport := func(f string, a ...interface{}) {
		fmt.Fprintf(reportWriter, f, a...)
	}

	dest, err := destRef.NewImageDestination(ctx)
	if err != nil {
		return fmt.Errorf("Error initializing destination %s: %v", transports.ImageName(destRef), err)
	}
	defer dest.Close()
	destSupportedManifestMIMETypes := dest.SupportedManifestMIMETypes()

	rawSource, err := srcRef.NewImageSource(ctx, destSupportedManifestMIMETypes)
	if err != nil {
		return fmt.Errorf("Error initializing source %s: %v", transports.ImageName(srcRef), err)
	}
	unparsedImage := image.UnparsedFromSource(rawSource)
	defer func() {
		if unparsedImage != nil {
			unparsedImage.Close()
		}
	}()

	// Please keep this policy check BEFORE reading any other information about the image.
	if allowed, err := policyContext.IsRunningImageAllowed(unparsedImage); !allowed || err != nil { // Be paranoid and fail if either return value indicates so.
		return fmt.Errorf("Source image rejected: %v", err)
	}
	src, err := image.FromUnparsedImage(unparsedImage)
	if err != nil {
		return fmt.Errorf("Error initializing image from source %s: %v", transports.ImageName(srcRef), err)
	}
	unparsedImage = nil
	defer src.Close()

	if src.IsMultiImage() {
		return fmt.Errorf("can not copy %s: manifest contains multiple images", transports.ImageName(srcRef))
	}

	var sigs [][]byte
	if options != nil && options.RemoveSignatures {
		sigs = [][]byte{}
	} else {
		writeReport("Getting image source signatures\n")
		s, err := src.Signatures()
		if err != nil {
			return fmt.Errorf("Error reading signatures: %v", err)
		}
		sigs = s
	}
	if len(sigs) != 0 {
		writeReport("Checking if image destination supports signatures\n")
		if err := dest.SupportsSignatures(); err != nil {
			return fmt.Errorf("Can not copy signatures: %v", err)
		}
	}

	canModifyManifest := len(sigs) == 0
	manifestUpdates := types.ManifestUpdateOptions{}

	if err := determineManifestConversion(&manifestUpdates, src, destSupportedManifestMIMETypes, canModifyManifest); err != nil {
		return err
	}

	if err := copyLayers(&manifestUpdates, dest, src, rawSource, canModifyManifest, reportWriter); err != nil {
		return err
	}

	pendingImage := src
	if !reflect.DeepEqual(manifestUpdates, types.ManifestUpdateOptions{InformationOnly: manifestUpdates.InformationOnly}) {
		if !canModifyManifest {
			return fmt.Errorf("Internal error: copy needs an updated manifest but that was known to be forbidden")
		}
		manifestUpdates.InformationOnly.Destination = dest
		pendingImage, err = src.UpdatedImage(manifestUpdates)
		if err != nil {
			return fmt.Errorf("Error creating an updated image manifest: %v", err)
		}
	}
	manifest, _, err := pendingImage.Manifest()
	if err != nil {
		return fmt.Errorf("Error reading manifest: %v", err)
	}

	if err := copyConfig(dest, pendingImage, reportWriter); err != nil {
		return err
	}

	if options != nil && options.SignBy != "" {
		mech, err := signature.NewGPGSigningMechanism()
		if err != nil {
			return fmt.Errorf("Error initializing GPG: %v", err)
		}
		dockerReference := dest.Reference().DockerReference()
		if dockerReference == nil {
			return fmt.Errorf("Cannot determine canonical Docker reference for destination %s", transports.ImageName(dest.Reference()))
		}

		writeReport("Signing manifest\n")
		newSig, err := signature.SignDockerManifest(manifest, dockerReference.String(), mech, options.SignBy)
		if err != nil {
			return fmt.Errorf("Error creating signature: %v", err)
		}
		sigs = append(sigs, newSig)
	}

	writeReport("Writing manifest to image destination\n")
	if err := dest.PutManifest(manifest); err != nil {
		return fmt.Errorf("Error writing manifest: %v", err)
	}

	writeReport("Storing signatures\n")
	if err := dest.PutSignatures(sigs); err != nil {
		return fmt.Errorf("Error writing signatures: %v", err)
	}

	if err := dest.Commit(); err != nil {
		return fmt.Errorf("Error committing the finished image: %v", err)
	}

	return nil
}

// copyLayers copies layers from src/rawSource to dest, using and updating manifestUpdates if necessary and canModifyManifest.
// If src.UpdatedImageNeedsLayerDiffIDs(manifestUpdates) will be true, it needs to be true by the time this function is called.
func copyLayers(manifestUpdates *types.ManifestUpdateOptions, dest types.ImageDestination, src types.Image, rawSource types.ImageSource,
	canModifyManifest bool, reportWriter io.Writer) error {
	type copiedLayer struct {
		blobInfo types.BlobInfo
		diffID   string
	}

	diffIDsAreNeeded := src.UpdatedImageNeedsLayerDiffIDs(*manifestUpdates)

	srcInfos := src.LayerInfos()
	destInfos := []types.BlobInfo{}
	diffIDs := []string{}
	copiedLayers := map[string]copiedLayer{}
	for _, srcLayer := range srcInfos {
		cl, ok := copiedLayers[srcLayer.Digest]
		if !ok {
			fmt.Fprintf(reportWriter, "Copying blob %s\n", srcLayer.Digest)
			destInfo, diffID, err := copyLayer(dest, rawSource, srcLayer, diffIDsAreNeeded, canModifyManifest, reportWriter)
			if err != nil {
				return err
			}
			cl = copiedLayer{blobInfo: destInfo, diffID: diffID}
			copiedLayers[srcLayer.Digest] = cl
		}
		destInfos = append(destInfos, cl.blobInfo)
		diffIDs = append(diffIDs, cl.diffID)
	}
	manifestUpdates.InformationOnly.LayerInfos = destInfos
	if diffIDsAreNeeded {
		manifestUpdates.InformationOnly.LayerDiffIDs = diffIDs
	}
	if layerDigestsDiffer(srcInfos, destInfos) {
		manifestUpdates.LayerInfos = destInfos
	}
	return nil
}

// layerDigestsDiffer return true iff the digests in a and b differ (ignoring sizes and possible other fields)
func layerDigestsDiffer(a, b []types.BlobInfo) bool {
	if len(a) != len(b) {
		return true
	}
	for i := range a {
		if a[i].Digest != b[i].Digest {
			return true
		}
	}
	return false
}

// copyConfig copies config.json, if any, from src to dest.
func copyConfig(dest types.ImageDestination, src types.Image, reportWriter io.Writer) error {
	srcInfo := src.ConfigInfo()
	if srcInfo.Digest != "" {
		fmt.Fprintf(reportWriter, "Copying config %s\n", srcInfo.Digest)
		configBlob, err := src.ConfigBlob()
		if err != nil {
			return fmt.Errorf("Error reading config blob %s: %v", srcInfo.Digest, err)
		}
		destInfo, err := copyBlobFromStream(dest, bytes.NewReader(configBlob), srcInfo, nil, false, reportWriter)
		if err != nil {
			return err
		}
		if destInfo.Digest != srcInfo.Digest {
			return fmt.Errorf("Internal error: copying uncompressed config blob %s changed digest to %s", srcInfo.Digest, destInfo.Digest)
		}
	}
	return nil
}

// diffIDResult contains both a digest value and an error from diffIDComputationGoroutine.
// We could also send the error through the pipeReader, but this more cleanly separates the copying of the layer and the DiffID computation.
type diffIDResult struct {
	digest string
	err    error
}

// copyLayer copies a layer with srcInfo (with known Digest and possibly known Size) in src to dest, perhaps compressing it if canCompress,
// and returns a complete blobInfo of the copied layer, and a value for LayerDiffIDs if diffIDIsNeeded
func copyLayer(dest types.ImageDestination, src types.ImageSource, srcInfo types.BlobInfo,
	diffIDIsNeeded bool, canCompress bool, reportWriter io.Writer) (types.BlobInfo, string, error) {
	srcStream, srcBlobSize, err := src.GetBlob(srcInfo.Digest) // We currently completely ignore srcInfo.Size throughout.
	if err != nil {
		return types.BlobInfo{}, "", fmt.Errorf("Error reading blob %s: %v", srcInfo.Digest, err)
	}
	defer srcStream.Close()

	blobInfo, diffIDChan, err := copyLayerFromStream(dest, srcStream, types.BlobInfo{Digest: srcInfo.Digest, Size: srcBlobSize},
		diffIDIsNeeded, canCompress, reportWriter)
	if err != nil {
		return types.BlobInfo{}, "", err
	}
	var diffIDResult diffIDResult // = {digest:""}
	if diffIDIsNeeded {
		diffIDResult = <-diffIDChan
		if diffIDResult.err != nil {
			return types.BlobInfo{}, "", fmt.Errorf("Error computing layer DiffID: %v", diffIDResult.err)
		}
		logrus.Debugf("Computed DiffID %s for layer %s", diffIDResult.digest, srcInfo.Digest)
	}
	return blobInfo, diffIDResult.digest, nil
}

// copyLayerFromStream is an implementation detail of copyLayer; mostly providing a separate “defer” scope.
// it copies a blob with srcInfo (with known Digest and possibly known Size) from srcStream to dest,
// perhaps compressing the stream if canCompress,
// and returns a complete blobInfo of the copied blob and perhaps a <-chan diffIDResult if diffIDIsNeeded, to be read by the caller.
func copyLayerFromStream(dest types.ImageDestination, srcStream io.Reader, srcInfo types.BlobInfo,
	diffIDIsNeeded bool, canCompress bool, reportWriter io.Writer) (types.BlobInfo, <-chan diffIDResult, error) {
	var getDiffIDRecorder func(decompressorFunc) io.Writer // = nil
	var diffIDChan chan diffIDResult

	err := errors.New("Internal error: unexpected panic in copyLayer") // For pipeWriter.CloseWithError below
	if diffIDIsNeeded {
		diffIDChan = make(chan diffIDResult, 1) // Buffered, so that sending a value after this or our caller has failed and exited does not block.
		pipeReader, pipeWriter := io.Pipe()
		defer func() { // Note that this is not the same as {defer pipeWriter.CloseWithError(err)}; we need err to be evaluated lazily.
			pipeWriter.CloseWithError(err) // CloseWithError(nil) is equivalent to Close()
		}()

		getDiffIDRecorder = func(decompressor decompressorFunc) io.Writer {
			// If this fails, e.g. because we have exited and due to pipeWriter.CloseWithError() above further
			// reading from the pipe has failed, we don’t really care.
			// We only read from diffIDChan if the rest of the flow has succeeded, and when we do read from it,
			// the return value includes an error indication, which we do check.
			//
			// If this gets never called, pipeReader will not be used anywhere, but pipeWriter will only be
			// closed above, so we are happy enough with both pipeReader and pipeWriter to just get collected by GC.
			go diffIDComputationGoroutine(diffIDChan, pipeReader, decompressor) // Closes pipeReader
			return pipeWriter
		}
	}
	blobInfo, err := copyBlobFromStream(dest, srcStream, srcInfo,
		getDiffIDRecorder, canCompress, reportWriter) // Sets err to nil on success
	return blobInfo, diffIDChan, err
	// We need the defer … pipeWriter.CloseWithError() to happen HERE so that the caller can block on reading from diffIDChan
}

// diffIDComputationGoroutine reads all input from pipeReader, uncompresses using decompressor if necessary, and sends its digest, and status, if any, to dest.
func diffIDComputationGoroutine(dest chan<- diffIDResult, pipeReader *io.PipeReader, decompressor decompressorFunc) {
	result := diffIDResult{
		digest: "",
		err:    errors.New("Internal error: unexpected panic in diffIDComputationGoroutine"),
	}
	defer func() { dest <- result }()
	defer pipeReader.Close() // We do not care to bother the other end of the pipe with other failures; we send them to dest instead.

	result.digest, result.err = computeDiffID(pipeReader, decompressor)
}

// computeDiffID reads all input from layerStream, uncompresses it using decompressor if necessary, and returns its digest.
func computeDiffID(stream io.Reader, decompressor decompressorFunc) (string, error) {
	if decompressor != nil {
		s, err := decompressor(stream)
		if err != nil {
			return "", err
		}
		stream = s
	}

	h := sha256.New()
	_, err := io.Copy(h, stream)
	if err != nil {
		return "", err
	}
	hash := h.Sum(nil)
	return "sha256:" + hex.EncodeToString(hash[:]), nil
}

// copyBlobFromStream copies a blob with srcInfo (with known Digest and possibly known Size) from srcStream to dest,
// perhaps sending a copy to an io.Writer if getOriginalLayerCopyWriter != nil,
// perhaps compressing it if canCompress,
// and returns a complete blobInfo of the copied blob.
func copyBlobFromStream(dest types.ImageDestination, srcStream io.Reader, srcInfo types.BlobInfo,
	getOriginalLayerCopyWriter func(decompressor decompressorFunc) io.Writer, canCompress bool,
	reportWriter io.Writer) (types.BlobInfo, error) {
	// The copying happens through a pipeline of connected io.Readers.
	// === Input: srcStream

	// === Process input through digestingReader to validate against the expected digest.
	// Be paranoid; in case PutBlob somehow managed to ignore an error from digestingReader,
	// use a separate validation failure indicator.
	// Note that we don't use a stronger "validationSucceeded" indicator, because
	// dest.PutBlob may detect that the layer already exists, in which case we don't
	// read stream to the end, and validation does not happen.
	digestingReader, err := newDigestingReader(srcStream, srcInfo.Digest)
	if err != nil {
		return types.BlobInfo{}, fmt.Errorf("Error preparing to verify blob %s: %v", srcInfo.Digest, err)
	}
	var destStream io.Reader = digestingReader

	// === Detect compression of the input stream.
	// This requires us to “peek ahead” into the stream to read the initial part, which requires us to chain through another io.Reader returned by detectCompression.
	decompressor, destStream, err := detectCompression(destStream) // We could skip this in some cases, but let's keep the code path uniform
	if err != nil {
		return types.BlobInfo{}, fmt.Errorf("Error reading blob %s: %v", srcInfo.Digest, err)
	}
	isCompressed := decompressor != nil

	// === Report progress using a pb.Reader.
	bar := pb.New(int(srcInfo.Size)).SetUnits(pb.U_BYTES)
	bar.Output = reportWriter
	bar.SetMaxWidth(80)
	bar.ShowTimeLeft = false
	bar.ShowPercent = false
	bar.Start()
	destStream = bar.NewProxyReader(destStream)
	defer fmt.Fprint(reportWriter, "\n")

	// === Send a copy of the original, uncompressed, stream, to a separate path if necessary.
	var originalLayerReader io.Reader // DO NOT USE this other than to drain the input if no other consumer in the pipeline has done so.
	if getOriginalLayerCopyWriter != nil {
		destStream = io.TeeReader(destStream, getOriginalLayerCopyWriter(decompressor))
		originalLayerReader = destStream
	}

	// === Compress the layer if it is uncompressed and compression is desired
	var inputInfo types.BlobInfo
	if !canCompress || isCompressed || !dest.ShouldCompressLayers() {
		logrus.Debugf("Using original blob without modification")
		inputInfo = srcInfo
	} else {
		logrus.Debugf("Compressing blob on the fly")
		pipeReader, pipeWriter := io.Pipe()
		defer pipeReader.Close()

		// If this fails while writing data, it will do pipeWriter.CloseWithError(); if it fails otherwise,
		// e.g. because we have exited and due to pipeReader.Close() above further writing to the pipe has failed,
		// we don’t care.
		go compressGoroutine(pipeWriter, destStream) // Closes pipeWriter
		destStream = pipeReader
		inputInfo.Digest = ""
		inputInfo.Size = -1
	}

	// === Finally, send the layer stream to dest.
	uploadedInfo, err := dest.PutBlob(destStream, inputInfo)
	if err != nil {
		return types.BlobInfo{}, fmt.Errorf("Error writing blob: %v", err)
	}

	// This is fairly horrible: the writer from getOriginalLayerCopyWriter wants to consumer
	// all of the input (to compute DiffIDs), even if dest.PutBlob does not need it.
	// So, read everything from originalLayerReader, which will cause the rest to be
	// sent there if we are not already at EOF.
	if getOriginalLayerCopyWriter != nil {
		logrus.Debugf("Consuming rest of the original blob to satisfy getOriginalLayerCopyWriter")
		_, err := io.Copy(ioutil.Discard, originalLayerReader)
		if err != nil {
			return types.BlobInfo{}, fmt.Errorf("Error reading input blob %s: %v", srcInfo.Digest, err)
		}
	}

	if digestingReader.validationFailed { // Coverage: This should never happen.
		return types.BlobInfo{}, fmt.Errorf("Internal error writing blob %s, digest verification failed but was ignored", srcInfo.Digest)
	}
	if inputInfo.Digest != "" && uploadedInfo.Digest != inputInfo.Digest {
		return types.BlobInfo{}, fmt.Errorf("Internal error writing blob %s, blob with digest %s saved with digest %s", srcInfo.Digest, inputInfo.Digest, uploadedInfo.Digest)
	}
	return uploadedInfo, nil
}

// compressGoroutine reads all input from src and writes its compressed equivalent to dest.
func compressGoroutine(dest *io.PipeWriter, src io.Reader) {
	err := errors.New("Internal error: unexpected panic in compressGoroutine")
	defer func() { // Note that this is not the same as {defer dest.CloseWithError(err)}; we need err to be evaluated lazily.
		dest.CloseWithError(err) // CloseWithError(nil) is equivalent to Close()
	}()

	zipper := gzip.NewWriter(dest)
	defer zipper.Close()

	_, err = io.Copy(zipper, src) // Sets err to nil, i.e. causes dest.Close()
}

// determineManifestConversion updates manifestUpdates to convert manifest to a supported MIME type, if necessary and canModifyManifest.
// Note that the conversion will only happen later, through src.UpdatedImage
func determineManifestConversion(manifestUpdates *types.ManifestUpdateOptions, src types.Image, destSupportedManifestMIMETypes []string, canModifyManifest bool) error {
	if len(destSupportedManifestMIMETypes) == 0 {
		return nil // Anything goes
	}
	supportedByDest := map[string]struct{}{}
	for _, t := range destSupportedManifestMIMETypes {
		supportedByDest[t] = struct{}{}
	}

	_, srcType, err := src.Manifest()
	if err != nil { // This should have been cached?!
		return fmt.Errorf("Error reading manifest: %v", err)
	}
	if _, ok := supportedByDest[srcType]; ok {
		logrus.Debugf("Manifest MIME type %s is declared supported by the destination", srcType)
		return nil
	}

	// OK, we should convert the manifest.
	if !canModifyManifest {
		logrus.Debugf("Manifest MIME type %s is not supported by the destination, but we can't modify the manifest, hoping for the best...")
		return nil // Take our chances - FIXME? Or should we fail without trying?
	}

	var chosenType = destSupportedManifestMIMETypes[0] // This one is known to be supported.
	for _, t := range preferredManifestMIMETypes {
		if _, ok := supportedByDest[t]; ok {
			chosenType = t
			break
		}
	}
	logrus.Debugf("Will convert manifest from MIME type %s to %s", srcType, chosenType)
	manifestUpdates.ManifestMIMEType = chosenType
	return nil
}
