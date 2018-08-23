package krpc

import (
	"encoding"
	"fmt"
	"reflect"

	"github.com/anacrolix/missinggo/slices"
	"github.com/anacrolix/torrent/bencode"
)

func unmarshalBencodedBinary(u encoding.BinaryUnmarshaler, b []byte) (err error) {
	var ub string
	err = bencode.Unmarshal(b, &ub)
	if err != nil {
		return
	}
	return u.UnmarshalBinary([]byte(ub))
}

type elemSizer interface {
	ElemSize() int
}

func unmarshalBinarySlice(slice elemSizer, b []byte) (err error) {
	sliceValue := reflect.ValueOf(slice).Elem()
	elemType := sliceValue.Type().Elem()
	bytesPerElem := slice.ElemSize()
	for len(b) != 0 {
		if len(b) < bytesPerElem {
			err = fmt.Errorf("%d trailing bytes < %d required for element", len(b), bytesPerElem)
			break
		}
		elem := reflect.New(elemType)
		err = elem.Interface().(encoding.BinaryUnmarshaler).UnmarshalBinary(b[:bytesPerElem])
		if err != nil {
			return
		}
		sliceValue.Set(reflect.Append(sliceValue, elem.Elem()))
		b = b[bytesPerElem:]
	}
	return
}

func marshalBinarySlice(slice elemSizer) (ret []byte, err error) {
	var elems []encoding.BinaryMarshaler
	slices.MakeInto(&elems, slice)
	for _, e := range elems {
		var b []byte
		b, err = e.MarshalBinary()
		if err != nil {
			return
		}
		if len(b) != slice.ElemSize() {
			panic(fmt.Sprintf("marshalled %d bytes, but expected %d", len(b), slice.ElemSize()))
		}
		ret = append(ret, b...)
	}
	return
}

func bencodeBytesResult(b []byte, err error) ([]byte, error) {
	if err != nil {
		return b, err
	}
	return bencode.Marshal(b)
}
