package tff

// Foo struct
type Foo struct {
	Blah int
}

// Record struct
type Record struct {
	Timestamp int64 `json:"id,omitempty"`
	OriginID  uint32
	Bar       Foo
	Method    string `json:"meth"`
	ReqID     string
	ServerIP  string
	RemoteIP  string
	BytesSent uint64
}
