package iter

// Callback receives a value and returns true if another value should be
// received or false to stop iteration.
type Callback func(value interface{}) (more bool)

// Func iterates by calling Callback for each of its values.
type Func func(Callback)

func All(cb Callback, fs ...Func) bool {
	for _, f := range fs {
		all := true
		f(func(v interface{}) bool {
			all = all && cb(v)
			return all
		})
		if !all {
			return false
		}
	}
	return true
}
