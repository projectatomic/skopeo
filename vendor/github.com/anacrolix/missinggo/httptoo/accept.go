package httptoo

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anacrolix/missinggo/mime"
)

func ParseAccept(line string) (parsed AcceptDirectives, err error) {
	dirs := strings.Split(line, ",")
	for _, d := range dirs {
		p := AcceptDirective{
			Q: 1,
		}
		ss := strings.Split(d, ";")
		switch len(ss) {
		case 2:
			p.Q, err = strconv.ParseFloat(ss[1], 32)
			if err != nil {
				return
			}
			fallthrough
		case 1:
			p.MimeType.FromString(ss[0])
		default:
			err = fmt.Errorf("error parsing %q", d)
			return
		}
		parsed = append(parsed, p)
	}
	return
}

type (
	AcceptDirectives []AcceptDirective
	AcceptDirective  struct {
		MimeType mime.Type
		Q        float64
	}
)
