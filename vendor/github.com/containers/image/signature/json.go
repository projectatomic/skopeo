package signature

import (
	"encoding/json"
	"fmt"
)

// jsonFormatError is returned when JSON does not match expected format.
type jsonFormatError string

func (err jsonFormatError) Error() string {
	return string(err)
}

// validateExactMapKeys returns an error if the keys of m are not exactly expectedKeys, which must be pairwise distinct
func validateExactMapKeys(m map[string]interface{}, expectedKeys ...string) error {
	if len(m) != len(expectedKeys) {
		return jsonFormatError("Unexpected keys in a JSON object")
	}

	for _, k := range expectedKeys {
		if _, ok := m[k]; !ok {
			return jsonFormatError(fmt.Sprintf("Key %s missing in a JSON object", k))
		}
	}
	// Assuming expectedKeys are pairwise distinct, we know m contains len(expectedKeys) different values in expectedKeys.
	return nil
}

// mapField returns a member fieldName of m, if it is a JSON map, or an error.
func mapField(m map[string]interface{}, fieldName string) (map[string]interface{}, error) {
	untyped, ok := m[fieldName]
	if !ok {
		return nil, jsonFormatError(fmt.Sprintf("Field %s missing", fieldName))
	}
	v, ok := untyped.(map[string]interface{})
	if !ok {
		return nil, jsonFormatError(fmt.Sprintf("Field %s is not a JSON object", fieldName))
	}
	return v, nil
}

// stringField returns a member fieldName of m, if it is a string, or an error.
func stringField(m map[string]interface{}, fieldName string) (string, error) {
	untyped, ok := m[fieldName]
	if !ok {
		return "", jsonFormatError(fmt.Sprintf("Field %s missing", fieldName))
	}
	v, ok := untyped.(string)
	if !ok {
		return "", jsonFormatError(fmt.Sprintf("Field %s is not a JSON object", fieldName))
	}
	return v, nil
}

// paranoidUnmarshalJSONObject unmarshals data as a JSON object, but failing on the slightest unexpected aspect
// (including duplicated keys, unrecognized keys, and non-matching types). Uses fieldResolver to
// determine the destination for a field value, which should return a pointer to the destination if valid, or nil if the key is rejected.
//
// The fieldResolver approach is useful for decoding the Policy.Transports map; using it for structs is a bit lazy,
// we could use reflection to automate this. Later?
func paranoidUnmarshalJSONObject(data []byte, fieldResolver func(string) interface{}) error {
	seenKeys := map[string]struct{}{}

	// NOTE: This is a go 1.4 implementation, very much non-paranoid! The json.Unmarshal below
	// already throws out duplicate keys.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return jsonFormatError(err.Error())
	}

	for key, valueJSON := range obj {
		if _, ok := seenKeys[key]; ok {
			return jsonFormatError(fmt.Sprintf("Duplicate key \"%s\"", key))
		}
		seenKeys[key] = struct{}{}

		valuePtr := fieldResolver(key)
		if valuePtr == nil {
			return jsonFormatError(fmt.Sprintf("Unknown key \"%s\"", key))
		}
		if err := json.Unmarshal(valueJSON, valuePtr); err != nil {
			return jsonFormatError(err.Error())
		}
	}
	return nil
}
