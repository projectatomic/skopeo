/**
 *  Copyright 2016 Paul Querna
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package types

import (
	"encoding/json"
	"reflect"
	"testing"

	ff "github.com/pquerna/ffjson/tests/number/ff"
)

func TestRoundTrip(t *testing.T) {
	var record ff.Number
	var recordTripped ff.Number
	ff.NewNumber(&record)

	buf1, err := json.Marshal(&record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	err = json.Unmarshal(buf1, &recordTripped)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	good := reflect.DeepEqual(record, recordTripped)
	if !good {
		t.Fatalf("Expected: %v\n Got: %v", record, recordTripped)
	}
}

func TestUnmarshalEmpty(t *testing.T) {
	record := ff.Number{}
	err := record.UnmarshalJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
}

const (
	numberJSON = `{
  "Int": 1,
  "Float": 3.14
}`
)

func TestUnmarshalFull(t *testing.T) {
	record := ff.Number{}
	err := record.UnmarshalJSON([]byte(numberJSON))
	if err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
}
