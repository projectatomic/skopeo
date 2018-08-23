package storage

import (
	"io"
	"os"
	"path/filepath"

	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/torrent/metainfo"
)

// File-based storage for torrents, that isn't yet bound to a particular
// torrent.
type fileClientImpl struct {
	baseDir   string
	pathMaker func(baseDir string, info *metainfo.Info, infoHash metainfo.Hash) string
	pc        PieceCompletion
}

// The Default path maker just returns the current path
func defaultPathMaker(baseDir string, info *metainfo.Info, infoHash metainfo.Hash) string {
	return baseDir
}

func infoHashPathMaker(baseDir string, info *metainfo.Info, infoHash metainfo.Hash) string {
	return filepath.Join(baseDir, infoHash.HexString())
}

// All Torrent data stored in this baseDir
func NewFile(baseDir string) ClientImpl {
	return NewFileWithCompletion(baseDir, pieceCompletionForDir(baseDir))
}

func NewFileWithCompletion(baseDir string, completion PieceCompletion) ClientImpl {
	return newFileWithCustomPathMakerAndCompletion(baseDir, nil, completion)
}

// File storage with data partitioned by infohash.
func NewFileByInfoHash(baseDir string) ClientImpl {
	return NewFileWithCustomPathMaker(baseDir, infoHashPathMaker)
}

// Allows passing a function to determine the path for storing torrent data
func NewFileWithCustomPathMaker(baseDir string, pathMaker func(baseDir string, info *metainfo.Info, infoHash metainfo.Hash) string) ClientImpl {
	return newFileWithCustomPathMakerAndCompletion(baseDir, pathMaker, pieceCompletionForDir(baseDir))
}

func newFileWithCustomPathMakerAndCompletion(baseDir string, pathMaker func(baseDir string, info *metainfo.Info, infoHash metainfo.Hash) string, completion PieceCompletion) ClientImpl {
	if pathMaker == nil {
		pathMaker = defaultPathMaker
	}
	return &fileClientImpl{
		baseDir:   baseDir,
		pathMaker: pathMaker,
		pc:        completion,
	}
}

func (me *fileClientImpl) Close() error {
	return me.pc.Close()
}

func (fs *fileClientImpl) OpenTorrent(info *metainfo.Info, infoHash metainfo.Hash) (TorrentImpl, error) {
	dir := fs.pathMaker(fs.baseDir, info, infoHash)
	err := CreateNativeZeroLengthFiles(info, dir)
	if err != nil {
		return nil, err
	}
	return &fileTorrentImpl{
		dir,
		info,
		infoHash,
		fs.pc,
	}, nil
}

type fileTorrentImpl struct {
	dir        string
	info       *metainfo.Info
	infoHash   metainfo.Hash
	completion PieceCompletion
}

func (fts *fileTorrentImpl) Piece(p metainfo.Piece) PieceImpl {
	// Create a view onto the file-based torrent storage.
	_io := fileTorrentImplIO{fts}
	// Return the appropriate segments of this.
	return &filePieceImpl{
		fts,
		p,
		missinggo.NewSectionWriter(_io, p.Offset(), p.Length()),
		io.NewSectionReader(_io, p.Offset(), p.Length()),
	}
}

func (fs *fileTorrentImpl) Close() error {
	return nil
}

// Creates natives files for any zero-length file entries in the info. This is
// a helper for file-based storages, which don't address or write to zero-
// length files because they have no corresponding pieces.
func CreateNativeZeroLengthFiles(info *metainfo.Info, dir string) (err error) {
	for _, fi := range info.UpvertedFiles() {
		if fi.Length != 0 {
			continue
		}
		name := filepath.Join(append([]string{dir, info.Name}, fi.Path...)...)
		os.MkdirAll(filepath.Dir(name), 0777)
		var f io.Closer
		f, err = os.Create(name)
		if err != nil {
			break
		}
		f.Close()
	}
	return
}

// Exposes file-based storage of a torrent, as one big ReadWriterAt.
type fileTorrentImplIO struct {
	fts *fileTorrentImpl
}

// Returns EOF on short or missing file.
func (fst *fileTorrentImplIO) readFileAt(fi metainfo.FileInfo, b []byte, off int64) (n int, err error) {
	f, err := os.Open(fst.fts.fileInfoName(fi))
	if os.IsNotExist(err) {
		// File missing is treated the same as a short file.
		err = io.EOF
		return
	}
	if err != nil {
		return
	}
	defer f.Close()
	// Limit the read to within the expected bounds of this file.
	if int64(len(b)) > fi.Length-off {
		b = b[:fi.Length-off]
	}
	for off < fi.Length && len(b) != 0 {
		n1, err1 := f.ReadAt(b, off)
		b = b[n1:]
		n += n1
		off += int64(n1)
		if n1 == 0 {
			err = err1
			break
		}
	}
	return
}

// Only returns EOF at the end of the torrent. Premature EOF is ErrUnexpectedEOF.
func (fst fileTorrentImplIO) ReadAt(b []byte, off int64) (n int, err error) {
	for _, fi := range fst.fts.info.UpvertedFiles() {
		for off < fi.Length {
			n1, err1 := fst.readFileAt(fi, b, off)
			n += n1
			off += int64(n1)
			b = b[n1:]
			if len(b) == 0 {
				// Got what we need.
				return
			}
			if n1 != 0 {
				// Made progress.
				continue
			}
			err = err1
			if err == io.EOF {
				// Lies.
				err = io.ErrUnexpectedEOF
			}
			return
		}
		off -= fi.Length
	}
	err = io.EOF
	return
}

func (fst fileTorrentImplIO) WriteAt(p []byte, off int64) (n int, err error) {
	for _, fi := range fst.fts.info.UpvertedFiles() {
		if off >= fi.Length {
			off -= fi.Length
			continue
		}
		n1 := len(p)
		if int64(n1) > fi.Length-off {
			n1 = int(fi.Length - off)
		}
		name := fst.fts.fileInfoName(fi)
		os.MkdirAll(filepath.Dir(name), 0777)
		var f *os.File
		f, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			return
		}
		n1, err = f.WriteAt(p[:n1], off)
		// TODO: On some systems, write errors can be delayed until the Close.
		f.Close()
		if err != nil {
			return
		}
		n += n1
		off = 0
		p = p[n1:]
		if len(p) == 0 {
			break
		}
	}
	return
}

func (fts *fileTorrentImpl) fileInfoName(fi metainfo.FileInfo) string {
	return filepath.Join(append([]string{fts.dir, fts.info.Name}, fi.Path...)...)
}
