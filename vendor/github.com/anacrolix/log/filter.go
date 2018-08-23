package log

type FilterFunc func(m *Msg) bool

type Filter struct {
	ff FilterFunc
}

func NewFilter(ff FilterFunc) *Filter {
	return &Filter{ff}
}
