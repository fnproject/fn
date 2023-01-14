package models

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

type Architecture []string

// Equals is defined based on un-ordered k/v comparison at of the annotation keys and (compacted) values of Architecture, JSON object-value equality for values is property-order dependent
func (m Architecture) Equals(other Architecture) bool {
	if len(m) != len(other) {
		return false
	}

	if len(m) != len(other) {
		return false
	}

	for _, arch1 := range m {
		found := false
		for _, arch2 := range other {
			found = arch1 == arch2
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func EmptyArchitecture() Architecture {
	return nil
}

// With Creates a new Architecture object containing the specified value - this does not perform size checks on the total number of keys
// this validates the correctness of the key and value. this returns a new the Architecture object with the key set.
func (m Architecture) With(value string) (Architecture, error) {

	if value == "" {
		return nil, errors.New("empty architecture value")
	}

	var newMd Architecture
	if m == nil {
		newMd = make(Architecture, 1)
	} else {
		newMd = m.clone()
	}
	newMd[0] = value
	return newMd, nil
}

// Validate validates a final Architecture object prior to store,
// This will reject partial/patch changes with empty values (containing deletes)
func (m Architecture) Validate() APIError {
	return nil
}

// clone produces a key-wise copy of the underlying Architecture
// publically MD can be copied by reference as it's (by contract) immutable
func (m Architecture) clone() Architecture {

	if m == nil {
		return nil
	}
	newMd := make(Architecture, len(m))
	for ok, ov := range m {
		newMd[ok] = ov
	}
	return newMd
}

// Value implements sql.Valuer, returning a string
func (m Architecture) Value() (driver.Value, error) {
	if len(m) < 1 {
		return driver.Value(string("")), nil
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(m)
	return driver.Value(b.String()), err
}

// Scan implements sql.Scanner
func (m *Architecture) Scan(value interface{}) error {
	if value == nil || value == "" {
		*m = nil
		return nil
	}
	bv, err := driver.String.ConvertValue(value)

	if err == nil {
		var b []byte
		switch x := bv.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
		}

		if len(b) > 0 {
			return json.Unmarshal(b, m)
		}

		*m = nil
		return nil
	}

	// otherwise, return an error
	return fmt.Errorf("Architecture invalid db format: %T %T value, err: %v", value, bv, err)
}
