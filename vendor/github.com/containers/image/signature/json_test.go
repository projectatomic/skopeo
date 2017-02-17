package signature

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mSI map[string]interface{} // To minimize typing the long name

// A short-hand way to get a JSON object field value or panic. No error handling done, we know
// what we are working with, a panic in a test is good enough, and fitting test cases on a single line
// is a priority.
func x(m mSI, fields ...string) mSI {
	for _, field := range fields {
		// Not .(mSI) because type assertion of an unnamed type to a named type always fails (the types
		// are not "identical"), but the assignment is fine because they are "assignable".
		m = m[field].(map[string]interface{})
	}
	return m
}

func TestValidateExactMapKeys(t *testing.T) {
	// Empty map and keys
	err := validateExactMapKeys(mSI{})
	assert.NoError(t, err)

	// Success
	err = validateExactMapKeys(mSI{"a": nil, "b": 1}, "b", "a")
	assert.NoError(t, err)

	// Extra map keys
	err = validateExactMapKeys(mSI{"a": nil, "b": 1}, "a")
	assert.Error(t, err)

	// Extra expected keys
	err = validateExactMapKeys(mSI{"a": 1}, "b", "a")
	assert.Error(t, err)

	// Unexpected key values
	err = validateExactMapKeys(mSI{"a": 1}, "b")
	assert.Error(t, err)
}

func TestInt64Field(t *testing.T) {
	// Field not found
	_, err := int64Field(mSI{"a": "x"}, "b")
	assert.Error(t, err)

	// Field has a wrong type
	_, err = int64Field(mSI{"a": "string"}, "a")
	assert.Error(t, err)

	for _, value := range []float64{
		0.5,         // Fractional input
		math.Inf(1), // Infinity
		math.NaN(),  // NaN
	} {
		_, err = int64Field(mSI{"a": value}, "a")
		assert.Error(t, err, fmt.Sprintf("%f", value))
	}

	// Success
	// The float64 type has 53 bits of effective precision, so ±1FFFFFFFFFFFFF is the
	// range of integer values which can all be represented exactly (beyond that,
	// some are representable if they are divisible by a high enough power of 2,
	// but most are not).
	for _, value := range []int64{0, 1, -1, 0x1FFFFFFFFFFFFF, -0x1FFFFFFFFFFFFF} {
		testName := fmt.Sprintf("%d", value)
		v, err := int64Field(mSI{"a": float64(value), "b": nil}, "a")
		require.NoError(t, err, testName)
		assert.Equal(t, value, v, testName)
	}
}

func TestMapField(t *testing.T) {
	// Field not found
	_, err := mapField(mSI{"a": mSI{}}, "b")
	assert.Error(t, err)

	// Field has a wrong type
	_, err = mapField(mSI{"a": 1}, "a")
	assert.Error(t, err)

	// Success
	// FIXME? We can't use mSI as the type of child, that type apparently can't be converted to the raw map type.
	child := map[string]interface{}{"b": mSI{}}
	m, err := mapField(mSI{"a": child, "b": nil}, "a")
	require.NoError(t, err)
	assert.Equal(t, child, m)
}

func TestStringField(t *testing.T) {
	// Field not found
	_, err := stringField(mSI{"a": "x"}, "b")
	assert.Error(t, err)

	// Field has a wrong type
	_, err = stringField(mSI{"a": 1}, "a")
	assert.Error(t, err)

	// Success
	s, err := stringField(mSI{"a": "x", "b": nil}, "a")
	require.NoError(t, err)
	assert.Equal(t, "x", s)
}

// implementsUnmarshalJSON is a minimalistic type used to detect that
// paranoidUnmarshalJSONObject uses the json.Unmarshaler interface of resolved
// pointers.
type implementsUnmarshalJSON bool

// Compile-time check that Policy implements json.Unmarshaler.
var _ json.Unmarshaler = (*implementsUnmarshalJSON)(nil)

func (dest *implementsUnmarshalJSON) UnmarshalJSON(data []byte) error {
	_ = data     // We don't care, not really.
	*dest = true // Mark handler as called
	return nil
}

func TestParanoidUnmarshalJSONObject(t *testing.T) {
	type testStruct struct {
		A string
		B int
	}
	ts := testStruct{}
	var unmarshalJSONCalled implementsUnmarshalJSON
	tsResolver := func(key string) interface{} {
		switch key {
		case "a":
			return &ts.A
		case "b":
			return &ts.B
		case "implementsUnmarshalJSON":
			return &unmarshalJSONCalled
		default:
			return nil
		}
	}

	// Empty object
	ts = testStruct{}
	err := paranoidUnmarshalJSONObject([]byte(`{}`), tsResolver)
	require.NoError(t, err)
	assert.Equal(t, testStruct{}, ts)

	// Success
	ts = testStruct{}
	err = paranoidUnmarshalJSONObject([]byte(`{"a":"x", "b":2}`), tsResolver)
	require.NoError(t, err)
	assert.Equal(t, testStruct{A: "x", B: 2}, ts)

	// json.Unamarshaler is used for decoding values
	ts = testStruct{}
	unmarshalJSONCalled = implementsUnmarshalJSON(false)
	err = paranoidUnmarshalJSONObject([]byte(`{"implementsUnmarshalJSON":true}`), tsResolver)
	require.NoError(t, err)
	assert.Equal(t, unmarshalJSONCalled, implementsUnmarshalJSON(true))

	// Various kinds of invalid input
	for _, input := range []string{
		``,                       // Empty input
		`&`,                      // Entirely invalid JSON
		`1`,                      // Not an object
		`{&}`,                    // Invalid key JSON
		`{1:1}`,                  // Key not a string
		`{"b":1, "b":1}`,         // Duplicate key
		`{"thisdoesnotexist":1}`, // Key rejected by resolver
		`{"a":&}`,                // Invalid value JSON
		`{"a":1}`,                // Type mismatch
		`{"a":"value"}{}`,        // Extra data after object
	} {
		ts = testStruct{}
		err := paranoidUnmarshalJSONObject([]byte(input), tsResolver)
		assert.Error(t, err, input)
	}
}
