package missinggo

import (
	"strings"

	"github.com/huandu/xstrings"
)

func KebabCase(s string) string {
	return strings.Replace(xstrings.ToSnakeCase(s), "_", "-", -1)
}
