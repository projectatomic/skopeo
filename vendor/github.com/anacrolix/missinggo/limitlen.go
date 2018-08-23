package missinggo

// Sets an upper bound on the len of b. max can be any type that will cast to
// int64.
func LimitLen(b []byte, max ...interface{}) []byte {
	return b[:MinInt(len(b), max...)]
}
