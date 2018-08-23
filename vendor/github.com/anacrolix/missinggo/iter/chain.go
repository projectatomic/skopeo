package iter

func Chain(fs ...Func) Func {
	return func(cb Callback) {
		for _, f := range fs {
			if !All(cb, f) {
				break
			}
		}
	}
}
