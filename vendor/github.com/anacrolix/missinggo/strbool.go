package missinggo

import (
	"strconv"
	"strings"
	"unicode"
)

func StringTruth(s string) (ret bool) {
	s = strings.TrimFunc(s, func(r rune) bool {
		return r == 0 || unicode.IsSpace(r)
	})
	if s == "" {
		return false
	}
	ret, err := strconv.ParseBool(s)
	if err == nil {
		return
	}
	i, err := strconv.ParseInt(s, 0, 0)
	if err == nil {
		return i != 0
	}
	return true
}
