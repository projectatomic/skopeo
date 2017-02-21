package storage

import (
	"encoding/base64"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// register all of the built-in drivers
	_ "github.com/containers/storage/drivers/register"

	drivers "github.com/containers/storage/drivers"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/containers/storage/pkg/stringid"
	"github.com/containers/storage/storageversion"
)

var (
	// ErrLoadError indicates that there was an initialization error.
	ErrLoadError = errors.New("error loading storage metadata")
	// ErrDuplicateID indicates that an ID which is to be assigned to a new item is already being used.
	ErrDuplicateID = errors.New("that ID is already in use")
	// ErrDuplicateName indicates that a name which is to be assigned to a new item is already being used.
	ErrDuplicateName = errors.New("that name is already in use")
	// ErrParentIsContainer is returned when a caller attempts to create a layer as a child of a container's layer.
	ErrParentIsContainer = errors.New("would-be parent layer is a container")
	// ErrNotAContainer is returned when the caller attempts to delete a container that isn't a container.
	ErrNotAContainer = errors.New("identifier is not a container")
	// ErrNotAnImage is returned when the caller attempts to delete an image that isn't an image.
	ErrNotAnImage = errors.New("identifier is not an image")
	// ErrNotALayer is returned when the caller attempts to delete a layer that isn't a layer.
	ErrNotALayer = errors.New("identifier is not a layer")
	// ErrNotAnID is returned when the caller attempts to read or write metadata from an item that doesn't exist.
	ErrNotAnID = errors.New("identifier is not a layer, image, or container")
	// ErrLayerHasChildren is returned when the caller attempts to delete a layer that has children.
	ErrLayerHasChildren = errors.New("layer has children")
	// ErrLayerUsedByImage is returned when the caller attempts to delete a layer that is an image's top layer.
	ErrLayerUsedByImage = errors.New("layer is in use by an image")
	// ErrLayerUsedByContainer is returned when the caller attempts to delete a layer that is a container's layer.
	ErrLayerUsedByContainer = errors.New("layer is in use by a container")
	// ErrImageUsedByContainer is returned when the caller attempts to delete an image that is a container's image.
	ErrImageUsedByContainer = errors.New("image is in use by a container")
	// ErrIncompleteOptions is returned when the caller attempts to initialize a Store without providing required information.
	ErrIncompleteOptions = errors.New("missing necessary StoreOptions")
	// ErrSizeUnknown is returned when the caller asks for the size of a big data item, but the Store couldn't determine the answer.
	ErrSizeUnknown = errors.New("size is not known")
	// DefaultStoreOptions is a reasonable default set of options.
	DefaultStoreOptions StoreOptions
	stores              []*store
	storesLock          sync.Mutex
)

// FileBasedStore wraps up the most common methods of the various types of file-based
// data stores that we implement.
type FileBasedStore interface {
	Locker

	// Load reloads the contents of the store from disk.  It should be called
	// with the lock held.
	Load() error

	// Save saves the contents of the store to disk.  It should be called with
	// the lock held, and Touch() should be called afterward before releasing the
	// lock.
	Save() error
}

// MetadataStore wraps up methods for getting and setting metadata associated with IDs.
type MetadataStore interface {
	// GetMetadata reads metadata associated with an item with the specified ID.
	GetMetadata(id string) (string, error)

	// SetMetadata updates the metadata associated with the item with the specified ID.
	SetMetadata(id, metadata string) error
}

// A BigDataStore wraps up the most common methods of the various types of
// file-based lookaside stores that we implement.
type BigDataStore interface {
	// SetBigData stores a (potentially large) piece of data associated with this
	// ID.
	SetBigData(id, key string, data []byte) error

	// GetBigData retrieves a (potentially large) piece of data associated with
	// this ID, if it has previously been set.
	GetBigData(id, key string) ([]byte, error)

	// GetBigDataSize retrieves the size of a (potentially large) piece of
	// data associated with this ID, if it has previously been set.
	GetBigDataSize(id, key string) (int64, error)

	// GetBigDataNames() returns a list of the names of previously-stored pieces of
	// data.
	GetBigDataNames(id string) ([]string, error)
}

// A FlaggableStore can have flags set and cleared on items which it manages.
type FlaggableStore interface {
	// ClearFlag removes a named flag from an item in the store.
	ClearFlag(id string, flag string) error

	// SetFlag sets a named flag and its value on an item in the store.
	SetFlag(id string, flag string, value interface{}) error
}

// StoreOptions is used for passing initialization options to GetStore(), for
// initializing a Store object and the underlying storage that it controls.
type StoreOptions struct {
	// RunRoot is the filesystem path under which we can store run-time
	// information, such as the locations of active mount points, that we
	// want to lose if the host is rebooted.
	RunRoot string `json:"runroot,omitempty"`
	// GraphRoot is the filesystem path under which we will store the
	// contents of layers, images, and containers.
	GraphRoot string `json:"root,omitempty"`
	// GraphDriverName is the underlying storage driver that we'll be
	// using.  It only needs to be specified the first time a Store is
	// initialized for a given RunRoot and GraphRoot.
	GraphDriverName string `json:"driver,omitempty"`
	// GraphDriverOptions are driver-specific options.
	GraphDriverOptions []string `json:"driver-options,omitempty"`
	// UIDMap and GIDMap are used mainly for deciding on the ownership of
	// files in layers as they're stored on disk, which is often necessary
	// when user namespaces are being used.
	UIDMap []idtools.IDMap `json:"uidmap,omitempty"`
	GIDMap []idtools.IDMap `json:"gidmap,omitempty"`
}

// Store wraps up the various types of file-based stores that we use into a
// singleton object that initializes and manages them all together.
type Store interface {
	// GetRunRoot, GetGraphRoot, GetGraphDriverName, and GetGraphOptions retrieve
	// settings that were passed to GetStore() when the object was created.
	GetRunRoot() string
	GetGraphRoot() string
	GetGraphDriverName() string
	GetGraphOptions() []string

	// GetGraphDriver obtains and returns a handle to the graph Driver object used
	// by the Store.
	GetGraphDriver() (drivers.Driver, error)

	// GetLayerStore obtains and returns a handle to the layer store object used by
	// the Store.
	GetLayerStore() (LayerStore, error)

	// GetImageStore obtains and returns a handle to the image store object used by
	// the Store.
	GetImageStore() (ImageStore, error)

	// GetContainerStore obtains and returns a handle to the container store object
	// used by the Store.
	GetContainerStore() (ContainerStore, error)

	// CreateLayer creates a new layer in the underlying storage driver, optionally
	// having the specified ID (one will be assigned if none is specified), with
	// the specified layer (or no layer) as its parent, and with optional names.
	// (The writeable flag is ignored.)
	CreateLayer(id, parent string, names []string, mountLabel string, writeable bool) (*Layer, error)

	// PutLayer combines the functions of CreateLayer and ApplyDiff, marking the
	// layer for automatic removal if applying the diff fails for any reason.
	PutLayer(id, parent string, names []string, mountLabel string, writeable bool, diff archive.Reader) (*Layer, int64, error)

	// CreateImage creates a new image, optionally with the specified ID
	// (one will be assigned if none is specified), with optional names,
	// referring to a specified image, and with optional metadata.  An
	// image is a record which associates the ID of a layer with a
	// additional bookkeeping information which the library stores for the
	// convenience of its caller.
	CreateImage(id string, names []string, layer, metadata string, options *ImageOptions) (*Image, error)

	// CreateContainer creates a new container, optionally with the specified ID
	// (one will be assigned if none is specified), with optional names,
	// using the specified image's top layer as the basis for the
	// container's layer, and assigning the specified ID to that layer (one
	// will be created if none is specified).  A container is a layer which
	// is associated with additional bookkeeping information which the
	// library stores for the convenience of its caller.
	CreateContainer(id string, names []string, image, layer, metadata string, options *ContainerOptions) (*Container, error)

	// GetMetadata retrieves the metadata which is associated with a layer, image,
	// or container (whichever the passed-in ID refers to).
	GetMetadata(id string) (string, error)

	// SetMetadata updates the metadata which is associated with a layer, image, or
	// container (whichever the passed-in ID refers to) to match the specified
	// value.  The metadata value can be retrieved at any time using GetMetadata,
	// or using GetLayer, GetImage, or GetContainer and reading the object directly.
	SetMetadata(id, metadata string) error

	// Exists checks if there is a layer, image, or container which has the
	// passed-in ID or name.
	Exists(id string) bool

	// Status asks for a status report, in the form of key-value pairs, from the
	// underlying storage driver.  The contents vary from driver to driver.
	Status() ([][2]string, error)

	// Delete removes the layer, image, or container which has the passed-in ID or
	// name.  Note that no safety checks are performed, so this can leave images
	// with references to layers which do not exist, and layers with references to
	// parents which no longer exist.
	Delete(id string) error

	// DeleteLayer attempts to remove the specified layer.  If the layer is the
	// parent of any other layer, or is referred to by any images, it will return
	// an error.
	DeleteLayer(id string) error

	// DeleteImage removes the specified image if it is not referred to by
	// any containers.  If its top layer is then no longer referred to by
	// any other images and is not the parent of any other layers, its top
	// layer will be removed.  If that layer's parent is no longer referred
	// to by any other images and is not the parent of any other layers,
	// then it, too, will be removed.  This procedure will be repeated
	// until a layer which should not be removed, or the base layer, is
	// reached, at which point the list of removed layers is returned.  If
	// the commit argument is false, the image and layers are not removed,
	// but the list of layers which would be removed is still returned.
	DeleteImage(id string, commit bool) (layers []string, err error)

	// DeleteContainer removes the specified container and its layer.  If there is
	// no matching container, or if the container exists but its layer does not, an
	// error will be returned.
	DeleteContainer(id string) error

	// Wipe removes all known layers, images, and containers.
	Wipe() error

	// Mount attempts to mount a layer, image, or container for access, and returns
	// the pathname if it succeeds.
	Mount(id, mountLabel string) (string, error)

	// Unmount attempts to unmount a layer, image, or container, given an ID, a
	// name, or a mount path.
	Unmount(id string) error

	// Changes returns a summary of the changes which would need to be made to one
	// layer to make its contents the same as a second layer.  If the first layer
	// is not specified, the second layer's parent is assumed.  Each Change
	// structure contains a Path relative to the layer's root directory, and a Kind
	// which is either ChangeAdd, ChangeModify, or ChangeDelete.
	Changes(from, to string) ([]archive.Change, error)

	// DiffSize returns a count of the size of the tarstream which would specify
	// the changes returned by Changes.
	DiffSize(from, to string) (int64, error)

	// Diff returns the tarstream which would specify the changes returned by
	// Changes.
	Diff(from, to string) (io.ReadCloser, error)

	// ApplyDiff applies a tarstream to a layer.  Information about the tarstream
	// is cached with the layer.  Typically, a layer which is populated using a
	// tarstream will be expected to not be modified in any other way, either
	// before or after the diff is applied.
	ApplyDiff(to string, diff archive.Reader) (int64, error)

	// Layers returns a list of the currently known layers.
	Layers() ([]Layer, error)

	// Images returns a list of the currently known images.
	Images() ([]Image, error)

	// Containers returns a list of the currently known containers.
	Containers() ([]Container, error)

	// GetNames returns the list of names for a layer, image, or container.
	GetNames(id string) ([]string, error)

	// SetNames changes the list of names for a layer, image, or container.
	SetNames(id string, names []string) error

	// ListImageBigData retrieves a list of the (possibly large) chunks of named
	// data associated with an image.
	ListImageBigData(id string) ([]string, error)

	// GetImageBigData retrieves a (possibly large) chunk of named data associated
	// with an image.
	GetImageBigData(id, key string) ([]byte, error)

	// GetImageBigDataSize retrieves the size of a (possibly large) chunk
	// of named data associated with an image.
	GetImageBigDataSize(id, key string) (int64, error)

	// SetImageBigData stores a (possibly large) chunk of named data associated
	// with an image.
	SetImageBigData(id, key string, data []byte) error

	// ListContainerBigData retrieves a list of the (possibly large) chunks of
	// named data associated with a container.
	ListContainerBigData(id string) ([]string, error)

	// GetContainerBigData retrieves a (possibly large) chunk of named data
	// associated with a container.
	GetContainerBigData(id, key string) ([]byte, error)

	// GetContainerBigDataSize retrieves the size of a (possibly large)
	// chunk of named data associated with a container.
	GetContainerBigDataSize(id, key string) (int64, error)

	// SetContainerBigData stores a (possibly large) chunk of named data
	// associated with a container.
	SetContainerBigData(id, key string, data []byte) error

	// GetLayer returns a specific layer.
	GetLayer(id string) (*Layer, error)

	// GetImage returns a specific image.
	GetImage(id string) (*Image, error)

	// GetImagesByTopLayer returns a list of images which reference the specified
	// layer as their top layer.  They will have different IDs and names
	// and may have different metadata, big data items, and flags.
	GetImagesByTopLayer(id string) ([]*Image, error)

	// GetContainer returns a specific container.
	GetContainer(id string) (*Container, error)

	// GetContainerByLayer returns a specific container based on its layer ID or
	// name.
	GetContainerByLayer(id string) (*Container, error)

	// GetContainerDirectory returns a path of a directory which the caller
	// can use to store data, specific to the container, which the library
	// does not directly manage.  The directory will be deleted when the
	// container is deleted.
	GetContainerDirectory(id string) (string, error)

	// SetContainerDirectoryFile is a convenience function which stores
	// a piece of data in the specified file relative to the container's
	// directory.
	SetContainerDirectoryFile(id, file string, data []byte) error

	// GetFromContainerDirectory is a convenience function which reads
	// the contents of the specified file relative to the container's
	// directory.
	GetFromContainerDirectory(id, file string) ([]byte, error)

	// GetContainerRunDirectory returns a path of a directory which the
	// caller can use to store data, specific to the container, which the
	// library does not directly manage.  The directory will be deleted
	// when the host system is restarted.
	GetContainerRunDirectory(id string) (string, error)

	// SetContainerRunDirectoryFile is a convenience function which stores
	// a piece of data in the specified file relative to the container's
	// run directory.
	SetContainerRunDirectoryFile(id, file string, data []byte) error

	// GetFromContainerRunDirectory is a convenience function which reads
	// the contents of the specified file relative to the container's run
	// directory.
	GetFromContainerRunDirectory(id, file string) ([]byte, error)

	// Lookup returns the ID of a layer, image, or container with the specified
	// name or ID.
	Lookup(name string) (string, error)

	// Shutdown attempts to free any kernel resources which are being used
	// by the underlying driver.  If "force" is true, any mounted (i.e., in
	// use) layers are unmounted beforehand.  If "force" is not true, then
	// layers being in use is considered to be an error condition.  A list
	// of still-mounted layers is returned along with possible errors.
	Shutdown(force bool) (layers []string, err error)

	// Version returns version information, in the form of key-value pairs, from
	// the storage package.
	Version() ([][2]string, error)
}

// ImageOptions is used for passing options to a Store's CreateImage() method.
type ImageOptions struct {
}

// ContainerOptions is used for passing options to a Store's CreateContainer() method.
type ContainerOptions struct {
}

type store struct {
	lastLoaded      time.Time
	runRoot         string
	graphLock       Locker
	graphRoot       string
	graphDriverName string
	graphOptions    []string
	uidMap          []idtools.IDMap
	gidMap          []idtools.IDMap
	graphDriver     drivers.Driver
	layerStore      LayerStore
	imageStore      ImageStore
	containerStore  ContainerStore
}

// GetStore attempts to find an already-created Store object matching the
// specified location and graph driver, and if it can't, it creates and
// initializes a new Store object, and the underlying storage that it controls.
func GetStore(options StoreOptions) (Store, error) {
	if options.RunRoot == "" && options.GraphRoot == "" && options.GraphDriverName == "" && len(options.GraphDriverOptions) == 0 {
		options = DefaultStoreOptions
	}

	if options.GraphRoot != "" {
		options.GraphRoot = filepath.Clean(options.GraphRoot)
	}
	if options.RunRoot != "" {
		options.RunRoot = filepath.Clean(options.RunRoot)
	}

	storesLock.Lock()
	defer storesLock.Unlock()

	for _, s := range stores {
		if s.graphRoot == options.GraphRoot && (options.GraphDriverName == "" || s.graphDriverName == options.GraphDriverName) {
			return s, nil
		}
	}

	if options.GraphRoot == "" {
		return nil, ErrIncompleteOptions
	}
	if options.RunRoot == "" {
		return nil, ErrIncompleteOptions
	}

	if err := os.MkdirAll(options.RunRoot, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	for _, subdir := range []string{} {
		if err := os.MkdirAll(filepath.Join(options.RunRoot, subdir), 0700); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}
	if err := os.MkdirAll(options.GraphRoot, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	for _, subdir := range []string{"mounts", "tmp", options.GraphDriverName} {
		if err := os.MkdirAll(filepath.Join(options.GraphRoot, subdir), 0700); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}

	graphLock, err := GetLockfile(filepath.Join(options.GraphRoot, "storage.lock"))
	if err != nil {
		return nil, err
	}
	s := &store{
		runRoot:         options.RunRoot,
		graphLock:       graphLock,
		graphRoot:       options.GraphRoot,
		graphDriverName: options.GraphDriverName,
		graphOptions:    options.GraphDriverOptions,
		uidMap:          copyIDMap(options.UIDMap),
		gidMap:          copyIDMap(options.GIDMap),
	}
	if err := s.load(); err != nil {
		return nil, err
	}

	stores = append(stores, s)

	return s, nil
}

func copyIDMap(idmap []idtools.IDMap) []idtools.IDMap {
	m := []idtools.IDMap{}
	if idmap != nil {
		m = make([]idtools.IDMap, len(idmap))
		copy(m, idmap)
	}
	if len(m) > 0 {
		return m[:]
	}
	return nil
}

func (s *store) GetRunRoot() string {
	return s.runRoot
}

func (s *store) GetGraphDriverName() string {
	return s.graphDriverName
}

func (s *store) GetGraphRoot() string {
	return s.graphRoot
}

func (s *store) GetGraphOptions() []string {
	return s.graphOptions
}

func (s *store) load() error {
	driver, err := s.GetGraphDriver()
	if err != nil {
		return err
	}
	s.graphDriver = driver
	s.graphDriverName = driver.String()
	driverPrefix := s.graphDriverName + "-"

	rls, err := s.GetLayerStore()
	if err != nil {
		return err
	}
	s.layerStore = rls

	gipath := filepath.Join(s.graphRoot, driverPrefix+"images")
	if err := os.MkdirAll(gipath, 0700); err != nil {
		return err
	}
	ris, err := newImageStore(gipath)
	if err != nil {
		return err
	}
	s.imageStore = ris
	gcpath := filepath.Join(s.graphRoot, driverPrefix+"containers")
	if err := os.MkdirAll(gcpath, 0700); err != nil {
		return err
	}
	rcs, err := newContainerStore(gcpath)
	if err != nil {
		return err
	}
	rcpath := filepath.Join(s.runRoot, driverPrefix+"containers")
	if err := os.MkdirAll(rcpath, 0700); err != nil {
		return err
	}
	s.containerStore = rcs
	return nil
}

func (s *store) getGraphDriver() (drivers.Driver, error) {
	if s.graphDriver != nil {
		return s.graphDriver, nil
	}
	driver, err := drivers.New(s.graphRoot, s.graphDriverName, s.graphOptions, s.uidMap, s.gidMap)
	if err != nil {
		return nil, err
	}
	s.graphDriver = driver
	s.graphDriverName = driver.String()
	return driver, nil
}

func (s *store) GetGraphDriver() (drivers.Driver, error) {
	s.graphLock.Lock()
	defer s.graphLock.Unlock()
	if s.graphLock.TouchedSince(s.lastLoaded) {
		s.graphDriver = nil
		s.layerStore = nil
		s.lastLoaded = time.Now()
	}
	return s.getGraphDriver()
}

func (s *store) GetLayerStore() (LayerStore, error) {
	s.graphLock.Lock()
	defer s.graphLock.Unlock()
	if s.graphLock.TouchedSince(s.lastLoaded) {
		s.graphDriver = nil
		s.layerStore = nil
		s.lastLoaded = time.Now()
	}
	if s.layerStore != nil {
		return s.layerStore, nil
	}
	driver, err := s.getGraphDriver()
	if err != nil {
		return nil, err
	}
	driverPrefix := s.graphDriverName + "-"
	rlpath := filepath.Join(s.runRoot, driverPrefix+"layers")
	if err := os.MkdirAll(rlpath, 0700); err != nil {
		return nil, err
	}
	glpath := filepath.Join(s.graphRoot, driverPrefix+"layers")
	if err := os.MkdirAll(glpath, 0700); err != nil {
		return nil, err
	}
	rls, err := newLayerStore(rlpath, glpath, driver)
	if err != nil {
		return nil, err
	}
	s.layerStore = rls
	return s.layerStore, nil
}

func (s *store) GetImageStore() (ImageStore, error) {
	if s.imageStore != nil {
		return s.imageStore, nil
	}
	return nil, ErrLoadError
}

func (s *store) GetContainerStore() (ContainerStore, error) {
	if s.containerStore != nil {
		return s.containerStore, nil
	}
	return nil, ErrLoadError
}

func (s *store) PutLayer(id, parent string, names []string, mountLabel string, writeable bool, diff archive.Reader) (*Layer, int64, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, -1, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, -1, err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return nil, -1, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	defer rlstore.Touch()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	defer ristore.Touch()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	defer rcstore.Touch()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}
	if id == "" {
		id = stringid.GenerateRandomID()
	}
	if parent != "" {
		if l, err := rlstore.Get(parent); err == nil && l != nil {
			parent = l.ID
		} else {
			return nil, -1, ErrLayerUnknown
		}
		containers, err := rcstore.Containers()
		if err != nil {
			return nil, -1, err
		}
		for _, container := range containers {
			if container.LayerID == parent {
				return nil, -1, ErrParentIsContainer
			}
		}
	}
	return rlstore.Put(id, parent, names, mountLabel, nil, writeable, nil, diff)
}

func (s *store) CreateLayer(id, parent string, names []string, mountLabel string, writeable bool) (*Layer, error) {
	layer, _, err := s.PutLayer(id, parent, names, mountLabel, writeable, nil)
	return layer, err
}

func (s *store) CreateImage(id string, names []string, layer, metadata string, options *ImageOptions) (*Image, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	defer ristore.Touch()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	defer rcstore.Touch()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}
	if id == "" {
		id = stringid.GenerateRandomID()
	}

	ilayer, err := rlstore.Get(layer)
	if err != nil {
		return nil, err
	}
	if ilayer == nil {
		return nil, ErrLayerUnknown
	}
	layer = ilayer.ID
	return ristore.Create(id, names, layer, metadata)
}

func (s *store) CreateContainer(id string, names []string, image, layer, metadata string, options *ContainerOptions) (*Container, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	defer rlstore.Touch()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	defer rcstore.Touch()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if id == "" {
		id = stringid.GenerateRandomID()
	}

	cimage, err := ristore.Get(image)
	if err != nil {
		return nil, err
	}
	if cimage == nil {
		return nil, ErrImageUnknown
	}
	clayer, err := rlstore.Create(layer, cimage.TopLayer, nil, "", nil, true)
	if err != nil {
		return nil, err
	}
	layer = clayer.ID
	container, err := rcstore.Create(id, names, cimage.ID, layer, metadata)
	if err != nil || container == nil {
		rlstore.Delete(layer)
	}
	return container, err
}

func (s *store) SetMetadata(id, metadata string) error {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if rlstore.Exists(id) {
		defer rlstore.Touch()
		return rlstore.SetMetadata(id, metadata)
	}
	if ristore.Exists(id) {
		defer ristore.Touch()
		return ristore.SetMetadata(id, metadata)
	}
	if rcstore.Exists(id) {
		defer rcstore.Touch()
		return rcstore.SetMetadata(id, metadata)
	}
	return ErrNotAnID
}

func (s *store) GetMetadata(id string) (string, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return "", err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return "", err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return "", err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if rlstore.Exists(id) {
		return rlstore.GetMetadata(id)
	}
	if ristore.Exists(id) {
		return ristore.GetMetadata(id)
	}
	if rcstore.Exists(id) {
		return rcstore.GetMetadata(id)
	}
	return "", ErrNotAnID
}

func (s *store) ListImageBigData(id string) ([]string, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}

	return ristore.GetBigDataNames(id)
}

func (s *store) GetImageBigDataSize(id, key string) (int64, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return -1, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return -1, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}

	return ristore.GetBigDataSize(id, key)
}

func (s *store) GetImageBigData(id, key string) ([]byte, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}

	return ristore.GetBigData(id, key)
}

func (s *store) SetImageBigData(id, key string, data []byte) error {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}

	return ristore.SetBigData(id, key, data)
}

func (s *store) ListContainerBigData(id string) ([]string, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	return rcstore.GetBigDataNames(id)
}

func (s *store) GetContainerBigDataSize(id, key string) (int64, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return -1, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return -1, err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return -1, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	return rcstore.GetBigDataSize(id, key)
}

func (s *store) GetContainerBigData(id, key string) ([]byte, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	return rcstore.GetBigData(id, key)
}

func (s *store) SetContainerBigData(id, key string, data []byte) error {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	return rcstore.SetBigData(id, key, data)
}

func (s *store) Exists(id string) bool {
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return false
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return false
	}
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return false
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if rcstore.Exists(id) {
		return true
	}
	if ristore.Exists(id) {
		return true
	}
	return rlstore.Exists(id)
}

func (s *store) SetNames(id string, names []string) error {
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return err
	}
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	deduped := []string{}
	seen := make(map[string]bool)
	for _, name := range names {
		if _, wasSeen := seen[name]; !wasSeen {
			seen[name] = true
			deduped = append(deduped, name)
		}
	}

	if rlstore.Exists(id) {
		return rlstore.SetNames(id, deduped)
	}
	if ristore.Exists(id) {
		return ristore.SetNames(id, deduped)
	}
	if rcstore.Exists(id) {
		return rcstore.SetNames(id, deduped)
	}
	return ErrLayerUnknown
}

func (s *store) GetNames(id string) ([]string, error) {
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return nil, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if l, err := rlstore.Get(id); l != nil && err == nil {
		return l.Names, nil
	}
	if i, err := ristore.Get(id); i != nil && err == nil {
		return i.Names, nil
	}
	if c, err := rcstore.Get(id); c != nil && err == nil {
		return c.Names, nil
	}
	return nil, ErrLayerUnknown
}

func (s *store) Lookup(name string) (string, error) {
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return "", err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return "", err
	}
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return "", err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if l, err := rlstore.Get(name); l != nil && err == nil {
		return l.ID, nil
	}
	if i, err := ristore.Get(name); i != nil && err == nil {
		return i.ID, nil
	}
	if c, err := rcstore.Get(name); c != nil && err == nil {
		return c.ID, nil
	}
	return "", ErrLayerUnknown
}

func (s *store) DeleteLayer(id string) error {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if rlstore.Exists(id) {
		defer rlstore.Touch()
		defer rcstore.Touch()
		if l, err := rlstore.Get(id); err != nil {
			id = l.ID
		}
		layers, err := rlstore.Layers()
		if err != nil {
			return err
		}
		for _, layer := range layers {
			if layer.Parent == id {
				return ErrLayerHasChildren
			}
		}
		images, err := ristore.Images()
		if err != nil {
			return err
		}
		for _, image := range images {
			if image.TopLayer == id {
				return ErrLayerUsedByImage
			}
		}
		containers, err := rcstore.Containers()
		if err != nil {
			return err
		}
		for _, container := range containers {
			if container.LayerID == id {
				return ErrLayerUsedByContainer
			}
		}
		return rlstore.Delete(id)
	}
	return ErrNotALayer
}

func (s *store) DeleteImage(id string, commit bool) (layers []string, err error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}
	layersToRemove := []string{}
	if ristore.Exists(id) {
		image, err := ristore.Get(id)
		if err != nil {
			return nil, err
		}
		id = image.ID
		defer rlstore.Touch()
		defer ristore.Touch()
		containers, err := rcstore.Containers()
		if err != nil {
			return nil, err
		}
		aContainerByImage := make(map[string]string)
		for _, container := range containers {
			aContainerByImage[container.ImageID] = container.ID
		}
		if _, ok := aContainerByImage[id]; ok {
			return nil, ErrImageUsedByContainer
		}
		images, err := ristore.Images()
		if err != nil {
			return nil, err
		}
		layers, err := rlstore.Layers()
		if err != nil {
			return nil, err
		}
		childrenByParent := make(map[string]*[]string)
		for _, layer := range layers {
			parent := layer.Parent
			if list, ok := childrenByParent[parent]; ok {
				newList := append(*list, layer.ID)
				childrenByParent[parent] = &newList
			} else {
				childrenByParent[parent] = &([]string{layer.ID})
			}
		}
		anyImageByTopLayer := make(map[string]string)
		for _, img := range images {
			if img.ID != id {
				anyImageByTopLayer[img.TopLayer] = img.ID
			}
		}
		if commit {
			if err = ristore.Delete(id); err != nil {
				return nil, err
			}
		}
		layer := image.TopLayer
		lastRemoved := ""
		for layer != "" {
			if rcstore.Exists(layer) {
				break
			}
			if _, ok := anyImageByTopLayer[layer]; ok {
				break
			}
			parent := ""
			if l, err := rlstore.Get(layer); err == nil {
				parent = l.Parent
			}
			otherRefs := 0
			if childList, ok := childrenByParent[layer]; ok && childList != nil {
				children := *childList
				for _, child := range children {
					if child != lastRemoved {
						otherRefs++
					}
				}
			}
			if otherRefs != 0 {
				break
			}
			lastRemoved = layer
			layersToRemove = append(layersToRemove, lastRemoved)
			layer = parent
		}
	} else {
		return nil, ErrNotAnImage
	}
	if commit {
		for _, layer := range layersToRemove {
			if err = rlstore.Delete(layer); err != nil {
				return nil, err
			}
		}
	}
	return layersToRemove, nil
}

func (s *store) DeleteContainer(id string) error {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if rcstore.Exists(id) {
		defer rlstore.Touch()
		defer rcstore.Touch()
		if container, err := rcstore.Get(id); err == nil {
			if rlstore.Exists(container.LayerID) {
				if err = rlstore.Delete(container.LayerID); err != nil {
					return err
				}
				if err = rcstore.Delete(id); err != nil {
					return err
				}
				middleDir := s.graphDriverName + "-containers"
				gcpath := filepath.Join(s.GetGraphRoot(), middleDir, container.ID)
				if err = os.RemoveAll(gcpath); err != nil {
					return err
				}
				rcpath := filepath.Join(s.GetRunRoot(), middleDir, container.ID)
				if err = os.RemoveAll(rcpath); err != nil {
					return err
				}
				return nil
			}
			return ErrNotALayer
		}
	}
	return ErrNotAContainer
}

func (s *store) Delete(id string) error {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if rcstore.Exists(id) {
		defer rlstore.Touch()
		defer rcstore.Touch()
		if container, err := rcstore.Get(id); err == nil {
			if rlstore.Exists(container.LayerID) {
				if err = rlstore.Delete(container.LayerID); err != nil {
					return err
				}
				if err = rcstore.Delete(id); err != nil {
					return err
				}
				middleDir := s.graphDriverName + "-containers"
				gcpath := filepath.Join(s.GetGraphRoot(), middleDir, container.ID, "userdata")
				if err = os.RemoveAll(gcpath); err != nil {
					return err
				}
				rcpath := filepath.Join(s.GetRunRoot(), middleDir, container.ID, "userdata")
				if err = os.RemoveAll(rcpath); err != nil {
					return err
				}
				return nil
			}
			return ErrNotALayer
		}
	}
	if ristore.Exists(id) {
		defer ristore.Touch()
		return ristore.Delete(id)
	}
	if rlstore.Exists(id) {
		defer rlstore.Touch()
		return rlstore.Delete(id)
	}
	return ErrLayerUnknown
}

func (s *store) Wipe() error {
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return err
	}
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	defer rlstore.Touch()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	defer ristore.Touch()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	defer rcstore.Touch()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if err = rcstore.Wipe(); err != nil {
		return err
	}
	if err = ristore.Wipe(); err != nil {
		return err
	}
	return rlstore.Wipe()
}

func (s *store) Status() ([][2]string, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	return rlstore.Status()
}

func (s *store) Version() ([][2]string, error) {
	return [][2]string{
		{"GitCommit", storageversion.GitCommit},
		{"Version", storageversion.Version},
		{"BuildTime", storageversion.BuildTime},
	}, nil
}

func (s *store) Mount(id, mountLabel string) (string, error) {
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return "", err
	}
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return "", err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	defer rlstore.Touch()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if c, err := rcstore.Get(id); c != nil && err == nil {
		id = c.LayerID
	}
	return rlstore.Mount(id, mountLabel)
}

func (s *store) Unmount(id string) error {
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return err
	}
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	defer rlstore.Touch()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	if c, err := rcstore.Get(id); c != nil && err == nil {
		id = c.LayerID
	}
	return rlstore.Unmount(id)
}

func (s *store) Changes(from, to string) ([]archive.Change, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}

	return rlstore.Changes(from, to)
}

func (s *store) DiffSize(from, to string) (int64, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return -1, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}

	return rlstore.DiffSize(from, to)
}

func (s *store) Diff(from, to string) (io.ReadCloser, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}

	return rlstore.Diff(from, to)
}

func (s *store) ApplyDiff(to string, diff archive.Reader) (int64, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return -1, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}

	return rlstore.ApplyDiff(to, diff)
}

func (s *store) Layers() ([]Layer, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}

	return rlstore.Layers()
}

func (s *store) Images() ([]Image, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}

	return ristore.Images()
}

func (s *store) Containers() ([]Container, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	return rcstore.Containers()
}

func (s *store) GetLayer(id string) (*Layer, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}

	return rlstore.Get(id)
}

func (s *store) GetImage(id string) (*Image, error) {
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}

	return ristore.Get(id)
}

func (s *store) GetImagesByTopLayer(id string) ([]*Image, error) {
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}

	layer, err := rlstore.Get(id)
	if err != nil {
		return nil, err
	}
	images := []*Image{}
	imageList, err := ristore.Images()
	if err != nil {
		return nil, err
	}
	for _, image := range imageList {
		if image.TopLayer == layer.ID {
			images = append(images, &image)
		}
	}

	return images, nil
}

func (s *store) GetContainer(id string) (*Container, error) {
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	return rcstore.Get(id)
}

func (s *store) GetContainerByLayer(id string) (*Container, error) {
	ristore, err := s.GetImageStore()
	if err != nil {
		return nil, err
	}
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return nil, err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return nil, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	layer, err := rlstore.Get(id)
	if err != nil {
		return nil, err
	}
	containerList, err := rcstore.Containers()
	if err != nil {
		return nil, err
	}
	for _, container := range containerList {
		if container.LayerID == layer.ID {
			return &container, nil
		}
	}

	return nil, ErrContainerUnknown
}

func (s *store) GetContainerDirectory(id string) (string, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return "", err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return "", err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return "", err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	id, err = rcstore.Lookup(id)
	if err != nil {
		return "", err
	}

	middleDir := s.graphDriverName + "-containers"
	gcpath := filepath.Join(s.GetGraphRoot(), middleDir, id, "userdata")
	if err := os.MkdirAll(gcpath, 0700); err != nil {
		return "", err
	}
	return gcpath, nil
}

func (s *store) GetContainerRunDirectory(id string) (string, error) {
	rlstore, err := s.GetLayerStore()
	if err != nil {
		return "", err
	}
	ristore, err := s.GetImageStore()
	if err != nil {
		return "", err
	}
	rcstore, err := s.GetContainerStore()
	if err != nil {
		return "", err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}
	ristore.Lock()
	defer ristore.Unlock()
	if modified, err := ristore.Modified(); modified || err != nil {
		ristore.Load()
	}
	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		rcstore.Load()
	}

	id, err = rcstore.Lookup(id)
	if err != nil {
		return "", err
	}

	middleDir := s.graphDriverName + "-containers"
	rcpath := filepath.Join(s.GetRunRoot(), middleDir, id, "userdata")
	if err := os.MkdirAll(rcpath, 0700); err != nil {
		return "", err
	}
	return rcpath, nil
}

func (s *store) SetContainerDirectoryFile(id, file string, data []byte) error {
	dir, err := s.GetContainerDirectory(id)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0700)
	if err != nil {
		return err
	}
	return ioutils.AtomicWriteFile(filepath.Join(dir, file), data, 0600)
}

func (s *store) GetFromContainerDirectory(id, file string) ([]byte, error) {
	dir, err := s.GetContainerDirectory(id)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadFile(filepath.Join(dir, file))
}

func (s *store) SetContainerRunDirectoryFile(id, file string, data []byte) error {
	dir, err := s.GetContainerRunDirectory(id)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0700)
	if err != nil {
		return err
	}
	return ioutils.AtomicWriteFile(filepath.Join(dir, file), data, 0600)
}

func (s *store) GetFromContainerRunDirectory(id, file string) ([]byte, error) {
	dir, err := s.GetContainerRunDirectory(id)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadFile(filepath.Join(dir, file))
}

func (s *store) Shutdown(force bool) ([]string, error) {
	mounted := []string{}
	modified := false

	rlstore, err := s.GetLayerStore()
	if err != nil {
		return mounted, err
	}

	rlstore.Lock()
	defer rlstore.Unlock()
	if modified, err := rlstore.Modified(); modified || err != nil {
		rlstore.Load()
	}

	s.graphLock.Lock()
	defer s.graphLock.Unlock()
	layers, err := rlstore.Layers()
	if err != nil {
		return mounted, err
	}
	for _, layer := range layers {
		if layer.MountCount == 0 {
			continue
		}
		mounted = append(mounted, layer.ID)
		if force {
			for layer.MountCount > 0 {
				err2 := rlstore.Unmount(layer.ID)
				if err2 != nil {
					if err == nil {
						err = err2
					}
					break
				}
				modified = true
			}
		}
	}
	if len(mounted) > 0 && err == nil {
		err = ErrLayerUsedByContainer
	}
	if err == nil {
		err = s.graphDriver.Cleanup()
		s.graphLock.Touch()
		modified = true
	}
	if modified {
		rlstore.Touch()
	}
	return mounted, err
}

// Convert a BigData key name into an acceptable file name.
func makeBigDataBaseName(key string) string {
	reader := strings.NewReader(key)
	for reader.Len() > 0 {
		ch, size, err := reader.ReadRune()
		if err != nil || size != 1 {
			break
		}
		if ch != '.' && !(ch >= '0' && ch <= '9') && !(ch >= 'a' && ch <= 'z') {
			break
		}
	}
	if reader.Len() > 0 {
		return "=" + base64.StdEncoding.EncodeToString([]byte(key))
	}
	return key
}

func init() {
	DefaultStoreOptions.RunRoot = "/var/run/containers"
	DefaultStoreOptions.GraphRoot = "/var/lib/containers"
	DefaultStoreOptions.GraphDriverName = os.Getenv("STORAGE_DRIVER")
	DefaultStoreOptions.GraphDriverOptions = strings.Split(os.Getenv("STORAGE_OPTS"), ",")
	if len(DefaultStoreOptions.GraphDriverOptions) == 1 && DefaultStoreOptions.GraphDriverOptions[0] == "" {
		DefaultStoreOptions.GraphDriverOptions = nil
	}
}
