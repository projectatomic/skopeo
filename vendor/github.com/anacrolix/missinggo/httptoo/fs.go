package httptoo

import (
	"net/http"
	"os"
)

// Wraps a http.FileSystem, disabling directory listings, per the commonly
// requested feature at https://groups.google.com/forum/#!topic/golang-
// nuts/bStLPdIVM6w .
type JustFilesFilesystem struct {
	Fs http.FileSystem
}

func (fs JustFilesFilesystem) Open(name string) (http.File, error) {
	f, err := fs.Fs.Open(name)
	if err != nil {
		return nil, err
	}
	d, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if d.IsDir() {
		f.Close()
		// This triggers http.FileServer to show a 404.
		return nil, os.ErrNotExist
	}
	return f, nil
}
