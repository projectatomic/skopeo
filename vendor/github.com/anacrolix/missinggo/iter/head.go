package iter

func Head(n int, f Func) Func {
	return func(cb Callback) {
		if n <= 0 {
			return
		}
		f(func(v interface{}) bool {
			n--
			if !cb(v) {
				return false
			}
			return n > 0
		})
	}
}
