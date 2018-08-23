package bencode

type Bytes []byte

var (
	_ Unmarshaler = &Bytes{}
	_ Marshaler   = &Bytes{}
	_ Marshaler   = Bytes{}
)

func (me *Bytes) UnmarshalBencode(b []byte) error {
	*me = append([]byte(nil), b...)
	return nil
}

func (me Bytes) MarshalBencode() ([]byte, error) {
	return me, nil
}
