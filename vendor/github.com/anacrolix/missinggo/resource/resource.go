package resource

import (
	"io"
	"os"
)

// An Instance represents the content at some location accessed through some
// Provider. It's the data at some URL.
type Instance interface {
	Get() (io.ReadCloser, error)
	Put(io.Reader) error
	Stat() (os.FileInfo, error)
	ReadAt([]byte, int64) (int, error)
	WriteAt([]byte, int64) (int, error)
	Delete() error
}

// Creates a io.ReadSeeker to an Instance.
func ReadSeeker(r Instance) io.ReadSeeker {
	fi, err := r.Stat()
	if err != nil {
		return nil
	}
	return io.NewSectionReader(r, 0, fi.Size())
}

// Move instance content, deleting the source if it succeeds.
func Move(from, to Instance) (err error) {
	rc, err := from.Get()
	if err != nil {
		return
	}
	defer rc.Close()
	err = to.Put(rc)
	if err != nil {
		return
	}
	from.Delete()
	return
}

func Exists(i Instance) bool {
	_, err := i.Stat()
	return err == nil
}
