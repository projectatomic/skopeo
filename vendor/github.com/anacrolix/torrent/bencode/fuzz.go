// +build gofuzz

package bencode

import (
	"fmt"
	"reflect"
)

func Fuzz(b []byte) int {
	var d interface{}
	err := Unmarshal(b, &d)
	if err != nil {
		return 0
	}
	b0, err := Marshal(d)
	if err != nil {
		panic(err)
	}
	var d0 interface{}
	err = Unmarshal(b0, &d0)
	if err != nil {
		panic(err)
	}
	if !reflect.DeepEqual(d, d0) {
		panic(fmt.Sprintf("%s != %s", d, d0))
	}
	return 1
}
