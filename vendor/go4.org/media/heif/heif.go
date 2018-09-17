/*
Copyright 2018 The go4 Authors

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

// Package heif reads HEIF containers, as found in Apple HEIC/HEVC images.
// This package does not decode images; it only reads the metadata.
//
// This package is a work in progress and makes no API compatibility
// promises.
package heif

import (
	"errors"
	"fmt"
	"io"
	"log"

	"go4.org/media/heif/bmff"
)

// File represents a HEIF file.
//
// Methods on File should not be called concurrently.
type File struct {
	ra      io.ReaderAt
	primary *Item

	// Populated lazily, by getMeta:
	metaErr error
	meta    *BoxMeta
}

// BoxMeta contains the low-level BMFF metadata boxes.
type BoxMeta struct {
	FileType     *bmff.FileTypeBox
	Handler      *bmff.HandlerBox
	PrimaryItem  *bmff.PrimaryItemBox
	ItemInfo     *bmff.ItemInfoBox
	Properties   *bmff.ItemPropertiesBox
	ItemLocation *bmff.ItemLocationBox
}

// EXIFItemID returns the item ID of the EXIF part, or 0 if not found.
func (m *BoxMeta) EXIFItemID() uint32 {
	if m.ItemInfo == nil {
		return 0
	}
	for _, ife := range m.ItemInfo.ItemInfos {
		if ife.ItemType == "Exif" {
			return uint32(ife.ItemID)
		}
	}
	return 0
}

// Item represents an item in a HEIF file.
type Item struct {
	f *File

	ID         uint32
	Info       *bmff.ItemInfoEntry
	Location   *bmff.ItemLocationBoxEntry // location in file
	Properties []bmff.Box
}

// SpatialExtents returns the item's spatial extents property values, if present,
// not correcting from any camera rotation metadata.
func (it *Item) SpatialExtents() (width, height int, ok bool) {
	for _, p := range it.Properties {
		if p, ok := p.(*bmff.ImageSpatialExtentsProperty); ok {
			return int(p.ImageWidth), int(p.ImageHeight), true
		}
	}
	return
}

// Rotations returns the number of 90 degree rotations counter-clockwise that this
// image should be rendered at, in the range [0,3].
func (it *Item) Rotations() int {
	for _, p := range it.Properties {
		if p, ok := p.(*bmff.ImageRotation); ok {
			return int(p.Angle)
		}
	}
	return 0
}

// VisualDimensions returns the item's width and height after correcting
// for any rotations.
func (it *Item) VisualDimensions() (width, height int, ok bool) {
	width, height, ok = it.SpatialExtents()
	for i := 0; i < it.Rotations(); i++ {
		width, height = height, width
	}
	return
}

// TODO: add HEIF imir (mirroring) accessor, like Image.SpatialExtents.

// Open returns a handle to access a HEIF file.
func Open(f io.ReaderAt) *File {
	return &File{ra: f}
}

// ErrNoEXIF is returned by File.EXIF when a file does not contain an EXIF item.
var ErrNoEXIF = errors.New("heif: no EXIF found")

// ErrUnknownItem is returned by File.ItemByID for unknown items.
var ErrUnknownItem = errors.New("heif: unknown item")

// EXIF returns the raw EXIF data from the file.
// The error is ErrNoEXIF if the file did not contain EXIF.
//
// The raw EXIF data can be parsed by the
// github.com/rwcarlsen/goexif/exif package's Decode function.
func (f *File) EXIF() ([]byte, error) {
	meta, err := f.getMeta()
	if err != nil {
		return nil, err
	}
	exifID := meta.EXIFItemID()
	if exifID == 0 {
		return nil, ErrNoEXIF
	}
	it, err := f.ItemByID(exifID)
	if err != nil {
		return nil, err
	}
	if it.Location == nil {
		return nil, errors.New("heif: file said it contained EXIF, but didn't say where")
	}
	if n := len(it.Location.Extents); n != 1 {
		return nil, fmt.Errorf("heif: expected 1 EXIF section, saw %d", n)
	}
	offLen := it.Location.Extents[0]
	const maxSize = 20 << 10 // 20MB of EXIF seems excessive; cap it for sanity
	if offLen.Length > maxSize {
		return nil, fmt.Errorf("heif: declared EXIF size %d exceeds threshold of %d bytes", offLen.Length, maxSize)
	}
	buf := make([]byte, offLen.Length-4)
	n, err := f.ra.ReadAt(buf, int64(offLen.Offset)+4) // TODO: why 4? did I miss something?
	if err != nil {
		log.Printf("Read %d bytes + %v: %q", n, err, buf)
		return nil, err
	}
	return buf, nil
}

func (f *File) setMetaErr(err error) error {
	if f.metaErr != nil {
		f.metaErr = err
	}
	return err
}

func (f *File) getMeta() (*BoxMeta, error) {
	if f.metaErr != nil {
		return nil, f.metaErr
	}
	if f.meta != nil {
		return f.meta, nil
	}
	const assumedMaxSize = 5 << 40 // arbitrary
	sr := io.NewSectionReader(f.ra, 0, assumedMaxSize)
	bmr := bmff.NewReader(sr)

	meta := &BoxMeta{}

	pbox, err := bmr.ReadAndParseBox(bmff.TypeFtyp)
	if err != nil {
		return nil, f.setMetaErr(err)
	}
	meta.FileType = pbox.(*bmff.FileTypeBox)

	pbox, err = bmr.ReadAndParseBox(bmff.TypeMeta)
	if err != nil {
		return nil, f.setMetaErr(err)
	}
	metabox := pbox.(*bmff.MetaBox)

	for _, box := range metabox.Children {
		boxp, err := box.Parse()
		if err == bmff.ErrUnknownBox {
			continue
		}
		if err != nil {
			return nil, f.setMetaErr(err)
		}
		switch v := boxp.(type) {
		case *bmff.HandlerBox:
			meta.Handler = v
		case *bmff.PrimaryItemBox:
			meta.PrimaryItem = v
		case *bmff.ItemInfoBox:
			meta.ItemInfo = v
		case *bmff.ItemPropertiesBox:
			meta.Properties = v
		case *bmff.ItemLocationBox:
			meta.ItemLocation = v
		}
	}

	f.meta = meta
	return f.meta, nil
}

// PrimaryItem returns the HEIF file's primary item.
func (f *File) PrimaryItem() (*Item, error) {
	meta, err := f.getMeta()
	if err != nil {
		return nil, err
	}
	if meta.PrimaryItem == nil {
		return nil, errors.New("heif: HEIF file lacks primary item box")
	}
	return f.ItemByID(uint32(meta.PrimaryItem.ItemID))
}

// ItemByID by returns the file's Item of a given ID.
// If the ID is known, the returned error is ErrUnknownItem.
func (f *File) ItemByID(id uint32) (*Item, error) {
	meta, err := f.getMeta()
	if err != nil {
		return nil, err
	}
	it := &Item{
		f:  f,
		ID: id,
	}
	if meta.ItemLocation != nil {
		for _, ilbe := range meta.ItemLocation.Items {
			if uint32(ilbe.ItemID) == id {
				shallowCopy := ilbe
				it.Location = &shallowCopy
			}
		}
	}
	if meta.ItemInfo != nil {
		for _, iie := range meta.ItemInfo.ItemInfos {
			if uint32(iie.ItemID) == id {
				it.Info = iie
			}
		}
	}
	if it.Info == nil {
		return nil, ErrUnknownItem
	}
	if meta.Properties != nil {
		allProps := meta.Properties.PropertyContainer.Properties
		for _, ipa := range meta.Properties.Associations {
			// TODO: I've never seen a file with more than
			// top-level ItemPropertyAssociation box, but
			// apparently they can exist with different
			// versions/flags. For now we just merge them
			// all together, but that's not really right.
			// So for now, just bail once a previous loop
			// found anything.
			if len(it.Properties) > 0 {
				break
			}

			for _, ipai := range ipa.Entries {
				if ipai.ItemID != id {
					continue
				}
				for _, ass := range ipai.Associations {
					if ass.Index != 0 && int(ass.Index) <= len(allProps) {
						box := allProps[ass.Index-1]
						boxp, err := box.Parse()
						if err == nil {
							box = boxp
						}
						it.Properties = append(it.Properties, box)
					}
				}
			}
		}
	}
	return it, nil
}
