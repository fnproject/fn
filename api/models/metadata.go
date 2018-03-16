package models

import (
	"encoding/json"
	"fmt"
	"bytes"
	"database/sql/driver"
	"errors"
)

// Metadata encapsulates key-value metadata. The structure is immutable via its public API and nil-safe for its contract
// permissive nilability is here to simplify updates and reduce the need for nil handling in extensions - metadata should be updated by over-writing the original object:
//  oldMd := EmptyMetadata()
//  newMd := oldMd.With("fooKey",1)
//  // old MD remains empty
// Metadata is lenable
type Metadata map[string]*metadataValue

// metadataValue encapsulates a metadata value in the metadata map,
// This is stored in its un-parsed JSON format for later (re-) parsing into specific structs or values
// metadataValue objects are  immutable after JSON load
type metadataValue struct {
	val []byte
}

const (
	maxMetadataValueBytes = 512
	maxMetadataKeyBytes   = 128
	maxMetadataKeys       = 100
)

// Equals is defined based on un-ordered k/v comparison at of the metadata keys and  (compacted) values of metadata, JSON object-value equality for values is property-order dependent
func (m Metadata) Equals(other Metadata) bool {
	if len(m) != len(other) {
		return false
	}
	for k1, v1 := range m {
		v2, _ := other[k1]
		if !bytes.Equal(v1.val, v2.val) {
			return false
		}
	}
	return true
}

func EmptyMetadata() Metadata {
	return nil
}

func (mv *metadataValue) String() string {
	return string(mv.val)
}

func (v *metadataValue) MarshalJSON() ([]byte, error) {
	return []byte(v.val), nil
}

func (v *metadataValue) isEmptyValue() bool {
	return string(v.val) == "\"\""
}

// UnmarshalJSON compacts metadata values but does not alter key-ordering for keys
func (v *metadataValue) UnmarshalJSON(val []byte) (error) {
	buf := bytes.Buffer{}
	err := json.Compact(&buf, val)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	v.val = buf.Bytes()
	return nil
}

func validateField(key string, value *metadataValue) APIError {

	if key == "" {
		return ErrEmptyMetadataKey
	}

	keyLen := len([]byte(key))

	if keyLen > maxMetadataKeyBytes {
		return ErrInvalidMetadataKeyLength
	}

	if value.isEmptyValue() {
		return ErrEmptyMetadataKey
	}

	if len(value.val) > maxMetadataValueBytes {
		return ErrInvalidMetadataValueLength
	}

	return nil
}

// With Creates a new Metadata object containing the specified value - this does not perform size checks on the total number of keys
// this validates the correctness of the key and value. this returns a new the metadata object with the key set.
func (m Metadata) With(key string, data interface{}) (Metadata, error) {

	if data == nil || data == "" {
		return nil, errors.New("empty metadata value")
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	newVal := &metadataValue{jsonBytes}
	err = validateField(key, newVal)

	if err != nil {
		return nil, err
	}

	var newMd Metadata
	if m == nil {
		newMd = make(Metadata)
	} else {
		newMd = m.Clone()
	}

	newMd[key] = newVal
	return newMd, nil
}

func (m Metadata) Validate() APIError {

	for k, v := range m {
		err := validateField(k, v)
		if err != nil {
			return err
		}
	}

	if len(m) > maxMetadataKeys {
		return ErrTooManyMetadataKeys
	}
	return nil
}

func (m Metadata) Get(key string) ([]byte, bool) {
	if v, ok := m[key]; ok {
		return v.val, ok
	}
	return nil, false

}

func (m Metadata) Without(key string) Metadata {
	nuVal := m.Clone()
	delete(nuVal, key)
	return nuVal
}

// MergeChange merges a delta (possibly including deletes) with an existing metadata object and returns a new (copy) metadata object or an error.
// This assumes that both old and new metadata objects contain only valid keys and only newVs may contain  deletes
func (m Metadata) MergeChange(newVs Metadata) Metadata {
	newMd := m.Clone()

	for k, v := range newVs {
		if v.isEmptyValue() {
			delete(newMd, k)
		} else {
			if newMd == nil {
				newMd = make(Metadata)
			}
			newMd[k] = v
		}
	}

	if len(newMd) == 0 {
		return EmptyMetadata()
	}
	return newMd
}

// Clone produces a key-wise copy of the map
func (m Metadata) Clone() Metadata {

	if m == nil {
		return nil
	}
	newMd := make(Metadata)
	for ok, ov := range m {
		newMd[ok] = ov
	}
	return newMd
}

// implements sql.Valuer, returning a string
func (m Metadata) Value() (driver.Value, error) {
	if len(m) < 1 {
		return driver.Value(string("")), nil
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(m)
	return driver.Value(b.String()), err
}

// implements sql.Scanner
func (m *Metadata) Scan(value interface{}) error {
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
	return fmt.Errorf("metadata invalid db format: %T %T value, err: %v", value, bv, err)
}
