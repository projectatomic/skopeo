package mime

import "strings"

type Type struct {
	Class    string
	Specific string
}

func (t Type) String() string {
	return t.Class + "/" + t.Specific
}

func (t *Type) FromString(s string) {
	ss := strings.SplitN(s, "/", 1)
	t.Class = ss[0]
	t.Specific = ss[1]
}
