package daemon

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
)

const temporaryDirectoryForBigFiles = "/var/tmp" // Do not use the system default of os.TempDir(), usually /tmp, because with systemd it could be a tmpfs.

type daemonImageSource struct {
	ref         daemonReference
	tarCopyPath string
	// The following data is only available after ensureCachedDataIsPresent() succeeds
	tarManifest       *manifestItem // nil if not available yet.
	configBytes       []byte
	configDigest      string
	orderedDiffIDList []diffID
	knownLayers       map[diffID]*layerInfo
	// Other state
	generatedManifest []byte // Private cache for GetManifest(), nil if not set yet.
}

type layerInfo struct {
	path string
	size int64
}

// newImageSource returns a types.ImageSource for the specified image reference.
// The caller must call .Close() on the returned ImageSource.
//
// It would be great if we were able to stream the input tar as it is being
// sent; but Docker sends the top-level manifest, which determines which paths
// to look for, at the end, so in we will need to seek back and re-read, several times.
// (We could, perhaps, expect an exact sequence, assume that the first plaintext file
// is the config, and that the following len(RootFS) files are the layers, but that feels
// way too brittle.)
func newImageSource(ctx *types.SystemContext, ref daemonReference) (types.ImageSource, error) {
	c, err := client.NewClient(client.DefaultDockerHost, "1.22", nil, nil) // FIXME: overridable host
	if err != nil {
		return nil, fmt.Errorf("Error initializing docker engine client: %v", err)
	}
	// Per NewReference(), ref.StringWithinTransport() is either an image ID (config digest), or a !reference.NameOnly() reference.
	// Either way ImageSave should create a tarball with exactly one image.
	inputStream, err := c.ImageSave(context.TODO(), []string{ref.StringWithinTransport()})
	if err != nil {
		return nil, fmt.Errorf("Error loading image from docker engine: %v", err)
	}
	defer inputStream.Close()

	// FIXME: use SystemContext here.
	tarCopyFile, err := ioutil.TempFile(temporaryDirectoryForBigFiles, "docker-daemon-tar")
	if err != nil {
		return nil, err
	}
	defer tarCopyFile.Close()

	succeeded := false
	defer func() {
		if !succeeded {
			os.Remove(tarCopyFile.Name())
		}
	}()

	if _, err := io.Copy(tarCopyFile, inputStream); err != nil {
		return nil, err
	}

	succeeded = true
	return &daemonImageSource{
		ref:         ref,
		tarCopyPath: tarCopyFile.Name(),
	}, nil
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *daemonImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *daemonImageSource) Close() {
	_ = os.Remove(s.tarCopyPath)
}

// tarReadCloser is a way to close the backing file of a tar.Reader when the user no longer needs the tar component.
type tarReadCloser struct {
	*tar.Reader
	backingFile *os.File
}

func (t *tarReadCloser) Close() error {
	return t.backingFile.Close()
}

// openTarComponent returns a ReadCloser for the specific file within the archive.
// This is linear scan; we assume that the tar file will have a fairly small amount of files (~layers),
// and that filesystem caching will make the repeated seeking over the (uncompressed) tarCopyPath cheap enough.
// The caller should call .Close() on the returned stream.
func (s *daemonImageSource) openTarComponent(componentPath string) (io.ReadCloser, error) {
	f, err := os.Open(s.tarCopyPath)
	if err != nil {
		return nil, err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			f.Close()
		}
	}()

	tarReader, header, err := findTarComponent(f, componentPath)
	if err != nil {
		return nil, err
	}
	if header == nil {
		return nil, os.ErrNotExist
	}
	if header.FileInfo().Mode()&os.ModeType == os.ModeSymlink { // FIXME: untested
		// We follow only one symlink; so no loops are possible.
		if _, err := f.Seek(0, os.SEEK_SET); err != nil {
			return nil, err
		}
		// The new path could easily point "outside" the archive, but we only compare it to existing tar headers without extracting the archive,
		// so we don't care.
		tarReader, header, err = findTarComponent(f, path.Join(path.Dir(componentPath), header.Linkname))
		if err != nil {
			return nil, err
		}
		if header == nil {
			return nil, os.ErrNotExist
		}
	}

	if !header.FileInfo().Mode().IsRegular() {
		return nil, fmt.Errorf("Error reading tar archive component %s: not a regular file", header.Name)
	}
	succeeded = true
	return &tarReadCloser{Reader: tarReader, backingFile: f}, nil
}

// findTarComponent returns a header and a reader matching path within inputFile,
// or (nil, nil, nil) if not found.
func findTarComponent(inputFile io.Reader, path string) (*tar.Reader, *tar.Header, error) {
	t := tar.NewReader(inputFile)
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if h.Name == path {
			return t, h, nil
		}
	}
	return nil, nil, nil
}

// readTarComponent returns full contents of componentPath.
func (s *daemonImageSource) readTarComponent(path string) ([]byte, error) {
	file, err := s.openTarComponent(path)
	if err != nil {
		return nil, fmt.Errorf("Error loading tar component %s: %v", path, err)
	}
	defer file.Close()
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// ensureCachedDataIsPresent loads data necessary for any of the public accessors.
func (s *daemonImageSource) ensureCachedDataIsPresent() error {
	if s.tarManifest != nil {
		return nil
	}

	// Read and parse manifest.json
	tarManifest, err := s.loadTarManifest()
	if err != nil {
		return err
	}

	// Read and parse config.
	configBytes, err := s.readTarComponent(tarManifest.Config)
	if err != nil {
		return err
	}
	var parsedConfig dockerImage // Most fields ommitted, we only care about layer DiffIDs.
	if err := json.Unmarshal(configBytes, &parsedConfig); err != nil {
		return fmt.Errorf("Error decoding tar config %s: %v", tarManifest.Config, err)
	}

	knownLayers, err := s.prepareLayerData(tarManifest, &parsedConfig)
	if err != nil {
		return err
	}

	// Success; commit.
	configHash := sha256.Sum256(configBytes)
	s.tarManifest = tarManifest
	s.configBytes = configBytes
	s.configDigest = "sha256:" + hex.EncodeToString(configHash[:])
	s.orderedDiffIDList = parsedConfig.RootFS.DiffIDs
	s.knownLayers = knownLayers
	return nil
}

// loadTarManifest loads and decodes the manifest.json.
func (s *daemonImageSource) loadTarManifest() (*manifestItem, error) {
	// FIXME? Do we need to deal with the legacy format?
	bytes, err := s.readTarComponent(manifestFileName)
	if err != nil {
		return nil, err
	}
	var items []manifestItem
	if err := json.Unmarshal(bytes, &items); err != nil {
		return nil, fmt.Errorf("Error decoding tar manifest.json: %v", err)
	}
	if len(items) != 1 {
		return nil, fmt.Errorf("Unexpected tar manifest.json: expected 1 item, got %d", len(items))
	}
	return &items[0], nil
}

func (s *daemonImageSource) prepareLayerData(tarManifest *manifestItem, parsedConfig *dockerImage) (map[diffID]*layerInfo, error) {
	// Collect layer data available in manifest and config.
	if len(tarManifest.Layers) != len(parsedConfig.RootFS.DiffIDs) {
		return nil, fmt.Errorf("Inconsistent layer count: %d in manifest, %d in config", len(tarManifest.Layers), len(parsedConfig.RootFS.DiffIDs))
	}
	knownLayers := map[diffID]*layerInfo{}
	unknownLayerSizes := map[string]*layerInfo{} // Points into knownLayers, a "to do list" of items with unknown sizes.
	for i, diffID := range parsedConfig.RootFS.DiffIDs {
		if _, ok := knownLayers[diffID]; ok {
			// Apparently it really can happen that a single image contains the same layer diff more than once.
			// In that case, the diffID validation ensures that both layers truly are the same, and it should not matter
			// which of the tarManifest.Layers paths is used; (docker save) actually makes the duplicates symlinks to the original.
			continue
		}
		layerPath := tarManifest.Layers[i]
		if _, ok := unknownLayerSizes[layerPath]; ok {
			return nil, fmt.Errorf("Layer tarfile %s used for two different DiffID values", layerPath)
		}
		li := &layerInfo{ // A new element in each iteration
			path: layerPath,
			size: -1,
		}
		knownLayers[diffID] = li
		unknownLayerSizes[layerPath] = li
	}

	// Scan the tar file to collect layer sizes.
	file, err := os.Open(s.tarCopyPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	t := tar.NewReader(file)
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if li, ok := unknownLayerSizes[h.Name]; ok {
			li.size = h.Size
			delete(unknownLayerSizes, h.Name)
		}
	}
	if len(unknownLayerSizes) != 0 {
		return nil, fmt.Errorf("Some layer tarfiles are missing in the tarball") // This could do with a better error reporting, if this ever happened in practice.
	}

	return knownLayers, nil
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
func (s *daemonImageSource) GetManifest() ([]byte, string, error) {
	if s.generatedManifest == nil {
		if err := s.ensureCachedDataIsPresent(); err != nil {
			return nil, "", err
		}
		m := schema2Manifest{
			SchemaVersion: 2,
			MediaType:     manifest.DockerV2Schema2MediaType,
			Config: distributionDescriptor{
				MediaType: manifest.DockerV2Schema2ConfigMediaType,
				Size:      int64(len(s.configBytes)),
				Digest:    s.configDigest,
			},
			Layers: []distributionDescriptor{},
		}
		for _, diffID := range s.orderedDiffIDList {
			li, ok := s.knownLayers[diffID]
			if !ok {
				return nil, "", fmt.Errorf("Internal inconsistency: Information about layer %s missing", diffID)
			}
			m.Layers = append(m.Layers, distributionDescriptor{
				Digest:    string(diffID), // diffID is a digest of the uncompressed tarball
				MediaType: manifest.DockerV2Schema2LayerMediaType,
				Size:      li.size,
			})
		}
		manifestBytes, err := json.Marshal(&m)
		if err != nil {
			return nil, "", err
		}
		s.generatedManifest = manifestBytes
	}
	return s.generatedManifest, manifest.DockerV2Schema2MediaType, nil
}

// GetTargetManifest returns an image's manifest given a digest. This is mainly used to retrieve a single image's manifest
// out of a manifest list.
func (s *daemonImageSource) GetTargetManifest(digest string) ([]byte, string, error) {
	// How did we even get here? GetManifest() above has returned a manifest.DockerV2Schema2MediaType.
	return nil, "", fmt.Errorf(`Manifest lists are not supported by "docker-daemon:"`)
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
func (s *daemonImageSource) GetBlob(digest string) (io.ReadCloser, int64, error) {
	if err := s.ensureCachedDataIsPresent(); err != nil {
		return nil, 0, err
	}

	if digest == s.configDigest { // FIXME? Implement a more general algorithm matching instead of assuming sha256.
		return ioutil.NopCloser(bytes.NewReader(s.configBytes)), int64(len(s.configBytes)), nil
	}

	if li, ok := s.knownLayers[diffID(digest)]; ok { // diffID is a digest of the uncompressed tarball,
		stream, err := s.openTarComponent(li.path)
		if err != nil {
			return nil, 0, err
		}
		return stream, li.size, nil
	}

	return nil, 0, fmt.Errorf("Unknown blob %s", digest)
}

// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
func (s *daemonImageSource) GetSignatures() ([][]byte, error) {
	return [][]byte{}, nil
}
