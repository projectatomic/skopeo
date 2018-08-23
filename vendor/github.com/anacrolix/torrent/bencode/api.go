package bencode

import (
	"bytes"
	"fmt"
	"io"
	"reflect"

	"github.com/anacrolix/missinggo/expect"
)

//----------------------------------------------------------------------------
// Errors
//----------------------------------------------------------------------------

// In case if marshaler cannot encode a type, it will return this error. Typical
// example of such type is float32/float64 which has no bencode representation.
type MarshalTypeError struct {
	Type reflect.Type
}

func (e *MarshalTypeError) Error() string {
	return "bencode: unsupported type: " + e.Type.String()
}

// Unmarshal argument must be a non-nil value of some pointer type.
type UnmarshalInvalidArgError struct {
	Type reflect.Type
}

func (e *UnmarshalInvalidArgError) Error() string {
	if e.Type == nil {
		return "bencode: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Ptr {
		return "bencode: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "bencode: Unmarshal(nil " + e.Type.String() + ")"
}

// Unmarshaler spotted a value that was not appropriate for a given Go value.
type UnmarshalTypeError struct {
	Value string
	Type  reflect.Type
}

func (e *UnmarshalTypeError) Error() string {
	return "bencode: value (" + e.Value + ") is not appropriate for type: " +
		e.Type.String()
}

// Unmarshaler tried to write to an unexported (therefore unwritable) field.
type UnmarshalFieldError struct {
	Key   string
	Type  reflect.Type
	Field reflect.StructField
}

func (e *UnmarshalFieldError) Error() string {
	return "bencode: key \"" + e.Key + "\" led to an unexported field \"" +
		e.Field.Name + "\" in type: " + e.Type.String()
}

// Malformed bencode input, unmarshaler failed to parse it.
type SyntaxError struct {
	Offset int64 // location of the error
	What   error // error description
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("bencode: syntax error (offset: %d): %s", e.Offset, e.What)
}

// A non-nil error was returned after calling MarshalBencode on a type which
// implements the Marshaler interface.
type MarshalerError struct {
	Type reflect.Type
	Err  error
}

func (e *MarshalerError) Error() string {
	return "bencode: error calling MarshalBencode for type " + e.Type.String() + ": " + e.Err.Error()
}

// A non-nil error was returned after calling UnmarshalBencode on a type which
// implements the Unmarshaler interface.
type UnmarshalerError struct {
	Type reflect.Type
	Err  error
}

func (e *UnmarshalerError) Error() string {
	return "bencode: error calling UnmarshalBencode for type " + e.Type.String() + ": " + e.Err.Error()
}

//----------------------------------------------------------------------------
// Interfaces
//----------------------------------------------------------------------------

// Any type which implements this interface, will be marshaled using the
// specified method.
type Marshaler interface {
	MarshalBencode() ([]byte, error)
}

// Any type which implements this interface, will be unmarshaled using the
// specified method.
type Unmarshaler interface {
	UnmarshalBencode([]byte) error
}

// Marshal the value 'v' to the bencode form, return the result as []byte and
// an error if any.
func Marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	e := Encoder{w: &buf}
	err := e.Encode(v)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func MustMarshal(v interface{}) []byte {
	b, err := Marshal(v)
	expect.Nil(err)
	return b
}

// Unmarshal the bencode value in the 'data' to a value pointed by the 'v'
// pointer, return a non-nil error if any.
func Unmarshal(data []byte, v interface{}) (err error) {
	buf := bytes.NewBuffer(data)
	e := Decoder{r: buf}
	err = e.Decode(v)
	if err == nil && buf.Len() != 0 {
		err = ErrUnusedTrailingBytes{buf.Len()}
	}
	return
}

type ErrUnusedTrailingBytes struct {
	NumUnusedBytes int
}

func (me ErrUnusedTrailingBytes) Error() string {
	return fmt.Sprintf("%d unused trailing bytes", me.NumUnusedBytes)
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: &scanner{r: r}}
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}
