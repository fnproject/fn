package models

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

type Architectures []string

func (m Architectures) Equals(other Architectures) bool {
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

func EmptyArchitecture() Architectures {
	return nil
}

func (m Architectures) With(value string) (Architectures, error) {

	if value == "" {
		return nil, errors.New("empty architecture value")
	}

	var newMd Architectures
	if m == nil {
		newMd = make(Architectures, 1)
	} else {
		newMd = m.clone()
	}
	newMd[0] = value
	return newMd, nil
}

// Validate validates a final Architecture object prior to store,
// This will reject partial/patch changes with empty values (containing deletes)
func (m Architectures) Validate() APIError {
	return nil
}

func (m Architectures) clone() Architectures {

	if m == nil {
		return nil
	}
	newMd := make(Architectures, len(m))
	for ok, ov := range m {
		newMd[ok] = ov
	}
	return newMd
}

// Value implements sql.Valuer, returning a string
func (m Architectures) Value() (driver.Value, error) {
	if len(m) < 1 {
		return driver.Value(string("")), nil
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(m)
	return driver.Value(b.String()), err
}

// Scan implements sql.Scanner
func (m *Architectures) Scan(value interface{}) error {
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
	return fmt.Errorf("Architectures invalid db format: %T %T value, err: %v", value, bv, err)
}
