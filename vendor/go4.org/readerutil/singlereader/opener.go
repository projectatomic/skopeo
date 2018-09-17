/*
Copyright 2013 The Go4 Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// package singlereader provides Open and Close operations, reusing existing
// file descriptors when possible.
package singlereader // import "go4.org/readerutil/singlereader"

import (
	"sync"

	"go4.org/readerutil"
	"go4.org/syncutil/singleflight"
	"go4.org/wkfs"
)

var (
	openerGroup singleflight.Group

	openFileMu sync.Mutex // guards openFiles
	openFiles  = make(map[string]*openFile)
)

type openFile struct {
	wkfs.File
	path     string // map key of openFiles
	refCount int
}

type openFileHandle struct {
	closed bool
	*openFile
}

func (f *openFileHandle) Close() error {
	openFileMu.Lock()
	if f.closed {
		openFileMu.Unlock()
		return nil
	}
	f.closed = true
	f.refCount--
	if f.refCount < 0 {
		panic("unexpected negative refcount")
	}
	zero := f.refCount == 0
	if zero {
		delete(openFiles, f.path)
	}
	openFileMu.Unlock()
	if !zero {
		return nil
	}
	return f.openFile.File.Close()
}

// Open opens the given file path for reading, reusing existing file descriptors
// when possible.
func Open(path string) (readerutil.ReaderAtCloser, error) {
	openFileMu.Lock()
	of := openFiles[path]
	if of != nil {
		of.refCount++
		openFileMu.Unlock()
		return &openFileHandle{false, of}, nil
	}
	openFileMu.Unlock() // release the lock while we call os.Open

	winner := false // this goroutine made it into Do's func

	// Returns an *openFile
	resi, err := openerGroup.Do(path, func() (interface{}, error) {
		winner = true
		f, err := wkfs.Open(path)
		if err != nil {
			return nil, err
		}
		of := &openFile{
			File:     f,
			path:     path,
			refCount: 1,
		}
		openFileMu.Lock()
		openFiles[path] = of
		openFileMu.Unlock()
		return of, nil
	})
	if err != nil {
		return nil, err
	}
	of = resi.(*openFile)

	// If our os.Open was dup-suppressed, we have to increment our
	// reference count.
	if !winner {
		openFileMu.Lock()
		if of.refCount == 0 {
			// Winner already closed it. Try again (rare).
			openFileMu.Unlock()
			return Open(path)
		}
		of.refCount++
		openFileMu.Unlock()
	}
	return &openFileHandle{false, of}, nil
}
