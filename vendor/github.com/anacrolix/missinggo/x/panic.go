package x

// Panic if error. Just fucking add exceptions, please.
func Pie(err error) {
	if err != nil {
		panic(err)
	}
}
