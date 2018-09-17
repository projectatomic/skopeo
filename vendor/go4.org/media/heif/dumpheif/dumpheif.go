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

// The dumpheif program dumps the structure and metadata of a HEIF file.
//
// It exists purely for debugging the go4.org/media/heif and
// go4.org/media/heif/bmff packages; it makes no backwards
// compatibility promises.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"

	"go4.org/media/heif"
	"go4.org/media/heif/bmff"
)

var (
	exifItemID uint16
	exifLoc    bmff.ItemLocationBoxEntry
)

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: dumpheif <file>\n")
		os.Exit(1)
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	hf := heif.Open(f)

	it, err := hf.PrimaryItem()
	if err != nil {
		log.Fatalf("PrimaryItem: %v", err)
	}
	fmt.Printf("primary item: %v\n", it.ID)

	width, height, ok := it.SpatialExtents()
	if ok {
		fmt.Printf("spatial extents: %d x %d\n", width, height)
	}
	fmt.Printf("properties:\n")
	for _, prop := range it.Properties {
		fmt.Printf("\t%q: %#v\n", prop.Type(), prop)
	}
	if len(it.Properties) == 0 {
		fmt.Printf("\t(no properties)\n")
	}

	if ex, err := hf.EXIF(); err == nil {
		fmt.Printf("EXIF dump:\n")
		ex, err := exif.Decode(bytes.NewReader(ex))
		if err != nil {
			log.Fatalf("EXIF decode: %v", err)
		}
		ex.Walk(exifWalkFunc(func(name exif.FieldName, tag *tiff.Tag) error {
			fmt.Printf("\t%v = %v\n", name, tag)
			return nil
		}))
		fmt.Printf("\n")
	}

	fmt.Printf("BMFF boxes:\n")
	r := bmff.NewReader(f)
	for {
		box, err := r.ReadBox()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("ReadBox: %v", err)
		}
		dumpBox(box, 0)
	}

}

type exifWalkFunc func(exif.FieldName, *tiff.Tag) error

func (f exifWalkFunc) Walk(name exif.FieldName, tag *tiff.Tag) error {
	return f(name, tag)
}

func dumpBox(box bmff.Box, depth int) {
	indent := strings.Repeat("    ", depth)
	fmt.Printf("%sBox: type %q, size %v\n", indent, box.Type(), box.Size())

	box2, err := box.Parse()
	if err == bmff.ErrUnknownBox {
		slurp, err := ioutil.ReadAll(box.Body())
		if err != nil {
			log.Fatalf("%sreading body: %v", indent, err)
		}
		if len(slurp) < 5000 {
			fmt.Printf("%s- contents: %q\n", indent, slurp)
		} else {
			fmt.Printf("%s- contents: (... %d bytes, starting with %q ...)\n", indent, len(slurp), slurp[:100])
		}
		return
	}
	if err != nil {
		slurp, _ := ioutil.ReadAll(box.Body())
		log.Fatalf("Parse box type %q: %v; slurp: %q", box.Type(), err, slurp)
	}

	switch v := box2.(type) {
	case *bmff.FileTypeBox, *bmff.HandlerBox, *bmff.PrimaryItemBox:
		fmt.Printf("%s- %T: %+v\n", indent, v, v)
	case *bmff.MetaBox:
		fmt.Printf("%s- %T, %d children:\n", indent, v, len(v.Children))
		for _, child := range v.Children {
			dumpBox(child, depth+1)
		}
	case *bmff.ItemInfoBox:
		//slurp, _ := ioutil.ReadAll(box.Body())
		//fmt.Printf("%s- %T raw: %q\n", indent, v, slurp)
		fmt.Printf("%s- %T, %d children (%d in slice):\n", indent, v, v.Count, len(v.ItemInfos))
		for _, child := range v.ItemInfos {
			dumpBox(child, depth+1)
		}
	case *bmff.ItemInfoEntry:
		fmt.Printf("%s- %T, %+v\n", indent, v, v)
		if v.ItemType == "Exif" {
			exifItemID = v.ItemID
		}
	case *bmff.ItemPropertiesBox:
		fmt.Printf("%s- %T\n", indent, v)
		if v.PropertyContainer != nil {
			dumpBox(v.PropertyContainer, depth+1)
		}
		for _, child := range v.Associations {
			dumpBox(child, depth+1)
		}
	case *bmff.ItemPropertyAssociation:
		fmt.Printf("%s- %T: %d declared entries, %d parsed:\n", indent, v, v.EntryCount, len(v.Entries))
		for _, ai := range v.Entries {
			fmt.Printf("%s  for Item ID %d, %d associations declared, %d parsed:\n", indent, ai.ItemID, ai.AssociationsCount, len(ai.Associations))
			for _, ass := range ai.Associations {
				fmt.Printf("%s    index: %d, essential: %v\n", indent, ass.Index, ass.Essential)
			}
		}
	case *bmff.DataInformationBox:
		fmt.Printf("%s- %T\n", indent, v)
		for _, child := range v.Children {
			dumpBox(child, depth+1)
		}
	case *bmff.DataReferenceBox:
		fmt.Printf("%s- %T\n", indent, v)
		for _, child := range v.Children {
			dumpBox(child, depth+1)
		}
	case *bmff.ItemPropertyContainerBox:
		fmt.Printf("%s- %T\n", indent, v)
		for _, child := range v.Properties {
			dumpBox(child, depth+1)
		}
	case *bmff.ItemLocationBox:
		fmt.Printf("%s- %T: %d items declared, %d parsed:\n", indent, v, v.ItemCount, len(v.Items))
		for _, lbe := range v.Items {
			fmt.Printf("%s  %+v\n", indent, lbe)
			if exifItemID != 0 && lbe.ItemID == exifItemID {
				exifLoc = lbe
			}
		}

	case *bmff.ImageSpatialExtentsProperty:
		fmt.Printf("%s- %T  dimensions: %d x %d\n", indent, v, v.ImageWidth, v.ImageHeight)
	default:
		fmt.Printf("%s- gotype: %T\n", indent, box2)
	}

}
