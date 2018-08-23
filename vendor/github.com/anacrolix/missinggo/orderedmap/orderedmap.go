package orderedmap

import "github.com/anacrolix/missinggo/iter"

func New(lesser func(l, r interface{}) bool) OrderedMap {
	return NewGoogleBTree(lesser)
}

type OrderedMap interface {
	Get(key interface{}) interface{}
	GetOk(key interface{}) (interface{}, bool)
	iter.Iterable
	Set(key, value interface{})
	Unset(key interface{})
	Len() int
}
