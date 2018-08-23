package bencode

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"runtime"
	"strconv"
	"sync"
)

type Decoder struct {
	r interface {
		io.ByteScanner
		io.Reader
	}
	// Sum of bytes used to Decode values.
	Offset int64
	buf    bytes.Buffer
}

func (d *Decoder) Decode(v interface{}) (err error) {
	defer func() {
		if err != nil {
			return
		}
		r := recover()
		_, ok := r.(runtime.Error)
		if ok {
			panic(r)
		}
		err, ok = r.(error)
		if !ok && r != nil {
			panic(r)
		}
	}()

	pv := reflect.ValueOf(v)
	if pv.Kind() != reflect.Ptr || pv.IsNil() {
		return &UnmarshalInvalidArgError{reflect.TypeOf(v)}
	}

	ok, err := d.parseValue(pv.Elem())
	if err != nil {
		return
	}
	if !ok {
		d.throwSyntaxError(d.Offset-1, errors.New("unexpected 'e'"))
	}
	return
}

func checkForUnexpectedEOF(err error, offset int64) {
	if err == io.EOF {
		panic(&SyntaxError{
			Offset: offset,
			What:   io.ErrUnexpectedEOF,
		})
	}
}

func (d *Decoder) readByte() byte {
	b, err := d.r.ReadByte()
	if err != nil {
		checkForUnexpectedEOF(err, d.Offset)
		panic(err)
	}

	d.Offset++
	return b
}

// reads data writing it to 'd.buf' until 'sep' byte is encountered, 'sep' byte
// is consumed, but not included into the 'd.buf'
func (d *Decoder) readUntil(sep byte) {
	for {
		b := d.readByte()
		if b == sep {
			return
		}
		d.buf.WriteByte(b)
	}
}

func checkForIntParseError(err error, offset int64) {
	if err != nil {
		panic(&SyntaxError{
			Offset: offset,
			What:   err,
		})
	}
}

func (d *Decoder) throwSyntaxError(offset int64, err error) {
	panic(&SyntaxError{
		Offset: offset,
		What:   err,
	})
}

// called when 'i' was consumed
func (d *Decoder) parseInt(v reflect.Value) {
	start := d.Offset - 1
	d.readUntil('e')
	if d.buf.Len() == 0 {
		panic(&SyntaxError{
			Offset: start,
			What:   errors.New("empty integer value"),
		})
	}

	s := bytesAsString(d.buf.Bytes())

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		checkForIntParseError(err, start)

		if v.OverflowInt(n) {
			panic(&UnmarshalTypeError{
				Value: "integer " + s,
				Type:  v.Type(),
			})
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		checkForIntParseError(err, start)

		if v.OverflowUint(n) {
			panic(&UnmarshalTypeError{
				Value: "integer " + s,
				Type:  v.Type(),
			})
		}
		v.SetUint(n)
	case reflect.Bool:
		v.SetBool(s != "0")
	default:
		panic(&UnmarshalTypeError{
			Value: "integer " + s,
			Type:  v.Type(),
		})
	}
	d.buf.Reset()
}

func (d *Decoder) parseString(v reflect.Value) error {
	start := d.Offset - 1

	// read the string length first
	d.readUntil(':')
	length, err := strconv.ParseInt(bytesAsString(d.buf.Bytes()), 10, 0)
	checkForIntParseError(err, start)

	defer d.buf.Reset()

	read := func(b []byte) {
		n, err := io.ReadFull(d.r, b)
		d.Offset += int64(n)
		if err != nil {
			checkForUnexpectedEOF(err, d.Offset)
			panic(&SyntaxError{
				Offset: d.Offset,
				What:   errors.New("unexpected I/O error: " + err.Error()),
			})
		}
	}

	switch v.Kind() {
	case reflect.String:
		b := make([]byte, length)
		read(b)
		v.SetString(bytesAsString(b))
		return nil
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			break
		}
		b := make([]byte, length)
		read(b)
		v.SetBytes(b)
		return nil
	case reflect.Array:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			break
		}
		d.buf.Grow(int(length))
		b := d.buf.Bytes()[:length]
		read(b)
		reflect.Copy(v, reflect.ValueOf(b))
		return nil
	}
	d.buf.Grow(int(length))
	read(d.buf.Bytes()[:length])
	// I believe we return here to support "ignore_unmarshal_type_error".
	return &UnmarshalTypeError{
		Value: "string",
		Type:  v.Type(),
	}
}

// Info for parsing a dict value.
type dictField struct {
	Value reflect.Value // Storage for the parsed value.
	// True if field value should be parsed into Value. If false, the value
	// should be parsed and discarded.
	Ok                       bool
	Set                      func() // Call this after parsing into Value.
	IgnoreUnmarshalTypeError bool
}

// Returns specifics for parsing a dict field value.
func getDictField(dict reflect.Value, key string) dictField {
	// get valuev as a map value or as a struct field
	switch dict.Kind() {
	case reflect.Map:
		value := reflect.New(dict.Type().Elem()).Elem()
		return dictField{
			Value: value,
			Ok:    true,
			Set: func() {
				if dict.IsNil() {
					dict.Set(reflect.MakeMap(dict.Type()))
				}
				// Assigns the value into the map.
				dict.SetMapIndex(reflect.ValueOf(key).Convert(dict.Type().Key()), value)
			},
		}
	case reflect.Struct:
		sf, ok := getStructFieldForKey(dict.Type(), key)
		if !ok {
			return dictField{}
		}
		if sf.r.PkgPath != "" {
			panic(&UnmarshalFieldError{
				Key:   key,
				Type:  dict.Type(),
				Field: sf.r,
			})
		}
		return dictField{
			Value: dict.FieldByIndex(sf.r.Index),
			Ok:    true,
			Set:   func() {},
			IgnoreUnmarshalTypeError: sf.tag.IgnoreUnmarshalTypeError(),
		}
	default:
		return dictField{}
	}
}

type structField struct {
	r   reflect.StructField
	tag tag
}

var (
	structFieldsMu sync.Mutex
	structFields   = map[reflect.Type]map[string]structField{}
)

func parseStructFields(struct_ reflect.Type, each func(string, structField)) {
	for i, n := 0, struct_.NumField(); i < n; i++ {
		f := struct_.Field(i)
		if f.Anonymous {
			continue
		}
		tagStr := f.Tag.Get("bencode")
		if tagStr == "-" {
			continue
		}
		tag := parseTag(tagStr)
		key := tag.Key()
		if key == "" {
			key = f.Name
		}
		each(key, structField{f, tag})
	}
}

func saveStructFields(struct_ reflect.Type) {
	m := make(map[string]structField)
	parseStructFields(struct_, func(key string, sf structField) {
		m[key] = sf
	})
	structFields[struct_] = m
}

func getStructFieldForKey(struct_ reflect.Type, key string) (f structField, ok bool) {
	structFieldsMu.Lock()
	if _, ok := structFields[struct_]; !ok {
		saveStructFields(struct_)
	}
	f, ok = structFields[struct_][key]
	structFieldsMu.Unlock()
	return
}

func (d *Decoder) parseDict(v reflect.Value) error {
	// so, at this point 'd' byte was consumed, let's just read key/value
	// pairs one by one
	for {
		var keyStr string
		keyValue := reflect.ValueOf(&keyStr).Elem()
		ok, err := d.parseValue(keyValue)
		if err != nil {
			return fmt.Errorf("error parsing dict key: %s", err)
		}
		if !ok {
			return nil
		}

		df := getDictField(v, keyStr)

		// now we need to actually parse it
		if df.Ok {
			// log.Printf("parsing ok struct field for key %q", keyStr)
			ok, err = d.parseValue(df.Value)
		} else {
			// Discard the value, there's nowhere to put it.
			var if_ interface{}
			if_, ok = d.parseValueInterface()
			if if_ == nil {
				err = fmt.Errorf("error parsing value for key %q", keyStr)
			}
		}
		if err != nil {
			if _, ok := err.(*UnmarshalTypeError); !ok || !df.IgnoreUnmarshalTypeError {
				return fmt.Errorf("parsing value for key %q: %s", keyStr, err)
			}
		}
		if !ok {
			return fmt.Errorf("missing value for key %q", keyStr)
		}
		if df.Ok {
			df.Set()
		}
	}
}

func (d *Decoder) parseList(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Array, reflect.Slice:
	default:
		panic(&UnmarshalTypeError{
			Value: "array",
			Type:  v.Type(),
		})
	}

	i := 0
	for ; ; i++ {
		if v.Kind() == reflect.Slice && i >= v.Len() {
			v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
		}

		if i < v.Len() {
			ok, err := d.parseValue(v.Index(i))
			if err != nil {
				return err
			}
			if !ok {
				break
			}
		} else {
			_, ok := d.parseValueInterface()
			if !ok {
				break
			}
		}
	}

	if i < v.Len() {
		if v.Kind() == reflect.Array {
			z := reflect.Zero(v.Type().Elem())
			for n := v.Len(); i < n; i++ {
				v.Index(i).Set(z)
			}
		} else {
			v.SetLen(i)
		}
	}

	if i == 0 && v.Kind() == reflect.Slice {
		v.Set(reflect.MakeSlice(v.Type(), 0, 0))
	}
	return nil
}

func (d *Decoder) readOneValue() bool {
	b, err := d.r.ReadByte()
	if err != nil {
		panic(err)
	}
	if b == 'e' {
		d.r.UnreadByte()
		return false
	} else {
		d.Offset++
		d.buf.WriteByte(b)
	}

	switch b {
	case 'd', 'l':
		// read until there is nothing to read
		for d.readOneValue() {
		}
		// consume 'e' as well
		b = d.readByte()
		d.buf.WriteByte(b)
	case 'i':
		d.readUntil('e')
		d.buf.WriteString("e")
	default:
		if b >= '0' && b <= '9' {
			start := d.buf.Len() - 1
			d.readUntil(':')
			length, err := strconv.ParseInt(bytesAsString(d.buf.Bytes()[start:]), 10, 64)
			checkForIntParseError(err, d.Offset-1)

			d.buf.WriteString(":")
			n, err := io.CopyN(&d.buf, d.r, length)
			d.Offset += n
			if err != nil {
				checkForUnexpectedEOF(err, d.Offset)
				panic(&SyntaxError{
					Offset: d.Offset,
					What:   errors.New("unexpected I/O error: " + err.Error()),
				})
			}
			break
		}

		d.raiseUnknownValueType(b, d.Offset-1)
	}

	return true

}

func (d *Decoder) parseUnmarshaler(v reflect.Value) bool {
	if !v.Type().Implements(unmarshalerType) {
		if v.Addr().Type().Implements(unmarshalerType) {
			v = v.Addr()
		} else {
			return false
		}
	}
	d.buf.Reset()
	if !d.readOneValue() {
		return false
	}
	m := v.Interface().(Unmarshaler)
	err := m.UnmarshalBencode(d.buf.Bytes())
	if err != nil {
		panic(&UnmarshalerError{v.Type(), err})
	}
	return true
}

// Returns true if there was a value and it's now stored in 'v', otherwise
// there was an end symbol ("e") and no value was stored.
func (d *Decoder) parseValue(v reflect.Value) (bool, error) {
	// we support one level of indirection at the moment
	if v.Kind() == reflect.Ptr {
		// if the pointer is nil, allocate a new element of the type it
		// points to
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	if d.parseUnmarshaler(v) {
		return true, nil
	}

	// common case: interface{}
	if v.Kind() == reflect.Interface && v.NumMethod() == 0 {
		iface, _ := d.parseValueInterface()
		v.Set(reflect.ValueOf(iface))
		return true, nil
	}

	b, err := d.r.ReadByte()
	if err != nil {
		panic(err)
	}
	d.Offset++

	switch b {
	case 'e':
		return false, nil
	case 'd':
		return true, d.parseDict(v)
	case 'l':
		return true, d.parseList(v)
	case 'i':
		d.parseInt(v)
		return true, nil
	default:
		if b >= '0' && b <= '9' {
			// It's a string.
			d.buf.Reset()
			// Write the first digit of the length to the buffer.
			d.buf.WriteByte(b)
			return true, d.parseString(v)
		}

		d.raiseUnknownValueType(b, d.Offset-1)
	}
	panic("unreachable")
}

// An unknown bencode type character was encountered.
func (d *Decoder) raiseUnknownValueType(b byte, offset int64) {
	panic(&SyntaxError{
		Offset: offset,
		What:   fmt.Errorf("unknown value type %+q", b),
	})
}

func (d *Decoder) parseValueInterface() (interface{}, bool) {
	b, err := d.r.ReadByte()
	if err != nil {
		panic(err)
	}
	d.Offset++

	switch b {
	case 'e':
		return nil, false
	case 'd':
		return d.parseDictInterface(), true
	case 'l':
		return d.parseListInterface(), true
	case 'i':
		return d.parseIntInterface(), true
	default:
		if b >= '0' && b <= '9' {
			// string
			// append first digit of the length to the buffer
			d.buf.WriteByte(b)
			return d.parseStringInterface(), true
		}

		d.raiseUnknownValueType(b, d.Offset-1)
		panic("unreachable")
	}
}

func (d *Decoder) parseIntInterface() (ret interface{}) {
	start := d.Offset - 1
	d.readUntil('e')
	if d.buf.Len() == 0 {
		panic(&SyntaxError{
			Offset: start,
			What:   errors.New("empty integer value"),
		})
	}

	n, err := strconv.ParseInt(d.buf.String(), 10, 64)
	if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
		i := new(big.Int)
		_, ok := i.SetString(d.buf.String(), 10)
		if !ok {
			panic(&SyntaxError{
				Offset: start,
				What:   errors.New("failed to parse integer"),
			})
		}
		ret = i
	} else {
		checkForIntParseError(err, start)
		ret = n
	}

	d.buf.Reset()
	return
}

func (d *Decoder) parseStringInterface() interface{} {
	start := d.Offset - 1

	// read the string length first
	d.readUntil(':')
	length, err := strconv.ParseInt(d.buf.String(), 10, 64)
	checkForIntParseError(err, start)

	d.buf.Reset()
	n, err := io.CopyN(&d.buf, d.r, length)
	d.Offset += n
	if err != nil {
		checkForUnexpectedEOF(err, d.Offset)
		panic(&SyntaxError{
			Offset: d.Offset,
			What:   errors.New("unexpected I/O error: " + err.Error()),
		})
	}

	s := d.buf.String()
	d.buf.Reset()
	return s
}

func (d *Decoder) parseDictInterface() interface{} {
	dict := make(map[string]interface{})
	for {
		keyi, ok := d.parseValueInterface()
		if !ok {
			break
		}

		key, ok := keyi.(string)
		if !ok {
			panic(&SyntaxError{
				Offset: d.Offset,
				What:   errors.New("non-string key in a dict"),
			})
		}

		valuei, ok := d.parseValueInterface()
		if !ok {
			break
		}

		dict[key] = valuei
	}
	return dict
}

func (d *Decoder) parseListInterface() interface{} {
	var list []interface{}
	for {
		valuei, ok := d.parseValueInterface()
		if !ok {
			break
		}

		list = append(list, valuei)
	}
	if list == nil {
		list = make([]interface{}, 0, 0)
	}
	return list
}
