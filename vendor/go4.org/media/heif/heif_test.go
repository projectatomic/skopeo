package heif

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
)

func TestAll(t *testing.T) {
	f, err := os.Open("testdata/park.heic")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := Open(f)

	// meta
	_, err = h.getMeta()
	if err != nil {
		t.Fatalf("getMeta: %v", err)
	}

	it, err := h.PrimaryItem()
	if err != nil {
		t.Fatalf("PrimaryItem: %v", err)
	}
	if want := uint32(49); it.ID != want {
		t.Errorf("PrimaryIem ID = %v; want %v", it.ID, want)
	}
	if it.Location == nil {
		t.Errorf("Item.Location is nil")
	}
	if it.Info == nil {
		t.Errorf("Item.Info is nil")
	}
	if len(it.Properties) == 0 {
		t.Errorf("Item.Properties is empty")
	}
	for _, prop := range it.Properties {
		t.Logf("  property: %q, %#v", prop.Type(), prop)
	}
	if w, h, ok := it.SpatialExtents(); !ok || w == 0 || h == 0 {
		t.Errorf("no spatial extents found")
	} else {
		t.Logf("dimensions: %v x %v", w, h)
	}

	// exif
	exbuf, err := h.EXIF()
	if err != nil {
		t.Errorf("EXIF: %v", err)
	} else {
		const magic = "Exif\x00\x00"
		if !bytes.HasPrefix(exbuf, []byte(magic)) {
			t.Errorf("Exif buffer doesn't start with %q: got %q", magic, exbuf)
		}
		x, err := exif.Decode(bytes.NewReader(exbuf))
		if err != nil {
			t.Fatalf("EXIF decode: %v", err)
		}
		got := map[string]string{}
		if err := x.Walk(walkFunc(func(name exif.FieldName, tag *tiff.Tag) error {
			got[fmt.Sprint(name)] = fmt.Sprint(tag)
			return nil
		})); err != nil {
			t.Fatalf("EXIF walk: %v", err)
		}
		if g, w := len(got), 56; g < w {
			t.Errorf("saw %v EXIF tags; want at least %v", g, w)
		}
		if g, w := got["GPSLongitude"], `["122/1","21/1","3776/100"]`; g != w {
			t.Errorf("GPSLongitude = %#q; want %#q", g, w)
		}

	}
}

func TestRotations(t *testing.T) {
	f, err := os.Open("testdata/rotate.heic")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := Open(f)
	it, err := h.PrimaryItem()
	if err != nil {
		t.Fatalf("PrimaryItem: %v", err)
	}
	if r := it.Rotations(); r != 3 {
		t.Errorf("Rotations = %v; want %v", r, 3)
	}
	sw, sh, ok := it.SpatialExtents()
	if !ok {
		t.Fatalf("expected spatial extents")
	}
	vw, vh, ok := it.VisualDimensions()
	if !ok {
		t.Fatalf("expected visual dimensions")
	}
	if vw != sh || vh != sw {
		t.Errorf("visual dimensions = %v, %v; want %v, %v", vw, vh, sh, sw)
	}
}

type walkFunc func(exif.FieldName, *tiff.Tag) error

func (f walkFunc) Walk(name exif.FieldName, tag *tiff.Tag) error {
	return f(name, tag)
}
