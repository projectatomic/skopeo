package httptoo

import (
	"fmt"
	"strings"
	"time"
)

type Visibility int

const (
	Default = 0
	Public  = 1
	Private = 2
)

type CacheControlHeader struct {
	MaxAge  time.Duration
	Caching Visibility
	NoStore bool
}

func (me *CacheControlHeader) caching() []string {
	switch me.Caching {
	case Public:
		return []string{"public"}
	case Private:
		return []string{"private"}
	default:
		return nil
	}
}

func (me *CacheControlHeader) maxAge() []string {
	if me.MaxAge == 0 {
		return nil
	}
	d := me.MaxAge
	if d < 0 {
		d = 0
	}
	return []string{fmt.Sprintf("max-age=%d", d/time.Second)}
}

func (me *CacheControlHeader) noStore() []string {
	if me.NoStore {
		return []string{"no-store"}
	}
	return nil
}

func (me *CacheControlHeader) concat(sss ...[]string) (ret []string) {
	for _, ss := range sss {
		ret = append(ret, ss...)
	}
	return
}

func (me CacheControlHeader) String() string {
	return strings.Join(me.concat(me.caching(), me.maxAge()), ", ")
}
