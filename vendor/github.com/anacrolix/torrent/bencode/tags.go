package bencode

import (
	"reflect"
	"strings"
)

func getTag(st reflect.StructTag) tag {
	return parseTag(st.Get("bencode"))
}

type tag []string

func parseTag(tagStr string) tag {
	return strings.Split(tagStr, ",")
}

func (me tag) Ignore() bool {
	return me[0] == "-"
}

func (me tag) Key() string {
	return me[0]
}

func (me tag) HasOpt(opt string) bool {
	for _, s := range me[1:] {
		if s == opt {
			return true
		}
	}
	return false
}

func (me tag) OmitEmpty() bool {
	return me.HasOpt("omitempty")
}

func (me tag) IgnoreUnmarshalTypeError() bool {
	return me.HasOpt("ignore_unmarshal_type_error")
}
