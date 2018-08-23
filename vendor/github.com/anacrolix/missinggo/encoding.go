package missinggo

// An interface for "encoding/base64".Encoder
type Encoding interface {
	EncodeToString([]byte) string
	DecodeString(string) ([]byte, error)
}

// An encoding that does nothing.
type IdentityEncoding struct{}

var _ Encoding = IdentityEncoding{}

func (IdentityEncoding) EncodeToString(b []byte) string        { return string(b) }
func (IdentityEncoding) DecodeString(s string) ([]byte, error) { return []byte(s), nil }
