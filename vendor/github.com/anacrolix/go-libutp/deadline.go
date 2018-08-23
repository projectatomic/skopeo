package utp

import "net"

type errDeadlineExceeded struct{}

var _ net.Error = errDeadlineExceeded{}

func (errDeadlineExceeded) Error() string   { return "deadline exceeded" }
func (errDeadlineExceeded) Temporary() bool { return false }
func (errDeadlineExceeded) Timeout() bool   { return true }
