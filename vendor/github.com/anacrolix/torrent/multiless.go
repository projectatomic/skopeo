package torrent

type (
	cmper     func() (same, less bool)
	multiLess struct {
		ok   bool
		less bool
	}
)

func (me *multiLess) Final() bool {
	if !me.ok {
		panic("undetermined")
	}
	return me.less
}

func (me *multiLess) FinalOk() (left, ok bool) {
	return me.less, me.ok
}

func (me *multiLess) Next(f cmper) {
	if me.ok {
		return
	}
	same, less := f()
	if same {
		return
	}
	me.ok = true
	me.less = less
}

func (me *multiLess) StrictNext(same, less bool) {
	if me.ok {
		return
	}
	me.Next(func() (bool, bool) { return same, less })
}

func (me *multiLess) NextBool(l, r bool) {
	me.StrictNext(l == r, l)
}
