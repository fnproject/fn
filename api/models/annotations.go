package models

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// Annotations encapsulates key-value metadata associated with resource. The structure is immutable via its public API and nil-safe for its contract
// permissive nilability is here to simplify updates and reduce the need for nil handling in extensions - annotations should be updated by over-writing the original object:
//
//	target.Annotations  = target.Annotations.With("fooKey",1)
//
// old MD remains empty
// Annotations is lenable
type Annotations map[string]*annotationValue

// annotationValue encapsulates a value in the annotations map,
// This is stored in its compacted, un-parsed JSON format for later (re-) parsing into specific structs or values
// annotationValue objects are  immutable after JSON load
type annotationValue []byte

const (
	maxAnnotationValueBytes = 512
	maxAnnotationKeyBytes   = 128
	maxAnnotationsKeys      = 100
)

// Equals is defined based on un-ordered k/v comparison at of the annotation keys and (compacted) values of annotations, JSON object-value equality for values is property-order dependent
func (m Annotations) Equals(other Annotations) bool {
	if len(m) != len(other) {
		return false
	}
	return m.Subset(other)
}

func (m Annotations) Subset(other Annotations) bool {
	for k1, v1 := range m {
		v2, _ := other[k1]
		if v2 == nil {
			return false
		}
		if !bytes.Equal(*v1, *v2) {
			return false
		}
	}
	return true
}

func EmptyAnnotations() Annotations {
	return nil
}

func (mv *annotationValue) String() string {
	return string(*mv)
}

func (v *annotationValue) MarshalJSON() ([]byte, error) {
	return *v, nil
}

func (mv *annotationValue) isEmptyValue() bool {
	sval := string(*mv)
	return sval == "\"\"" || sval == "null"
}

// UnmarshalJSON compacts annotation values but does not alter key-ordering for keys
func (mv *annotationValue) UnmarshalJSON(val []byte) error {
	buf := bytes.Buffer{}
	err := json.Compact(&buf, val)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	*mv = buf.Bytes()
	return nil
}

var validKeyRegex = regexp.MustCompile("^[!-~]+$")

func validateField(key string, value annotationValue) APIError {

	if !validKeyRegex.Match([]byte(key)) {
		return ErrInvalidAnnotationKey
	}

	keyLen := len([]byte(key))

	if keyLen > maxAnnotationKeyBytes {
		return ErrInvalidAnnotationKeyLength
	}

	if value.isEmptyValue() {
		return ErrInvalidAnnotationValue
	}

	if len(value) > maxAnnotationValueBytes {
		return ErrInvalidAnnotationValueLength
	}

	return nil
}

// With Creates a new annotations object containing the specified value - this does not perform size checks on the total number of keys
// this validates the correctness of the key and value. this returns a new the annotations object with the key set.
func (m Annotations) With(key string, data interface{}) (Annotations, error) {

	if data == nil || data == "" {
		return nil, errors.New("empty annotation value")
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	newVal := jsonBytes
	err = validateField(key, newVal)

	if err != nil {
		return nil, err
	}

	var newMd Annotations
	if m == nil {
		newMd = make(Annotations, 1)
	} else {
		newMd = m.clone()
	}
	mv := annotationValue(newVal)
	newMd[key] = &mv
	return newMd, nil
}

// Validate validates a final annotations object prior to store,
// This will reject partial/patch changes with empty values (containing deletes)
func (m Annotations) Validate() APIError {

	for k, v := range m {
		err := validateField(k, *v)
		if err != nil {
			return err
		}
	}

	if len(m) > maxAnnotationsKeys {
		return ErrTooManyAnnotationKeys
	}
	return nil
}

// Get returns a raw JSON value of a annotation key
func (m Annotations) Get(key string) ([]byte, bool) {
	if v, ok := m[key]; ok {
		return *v, ok
	}
	return nil, false
}

// GetString returns a string value if the annotation value is a string, otherwise an error
func (m Annotations) GetString(key string) (string, error) {
	if v, ok := m[key]; ok {
		var s string
		if err := json.Unmarshal([]byte(*v), &s); err != nil {
			return "", err
		}
		return s, nil
	}
	return "", errors.New("Annotation not found")
}

// Without returns a new annotations object with a value excluded
func (m Annotations) Without(key string) Annotations {
	nuVal := m.clone()
	delete(nuVal, key)
	return nuVal
}

// MergeChange merges a delta (possibly including deletes) with an existing annotations object and returns a new (copy) annotations object or an error.
// This assumes that both old and new annotations objects contain only valid keys and only newVs may contain  deletes
func (m Annotations) MergeChange(newVs Annotations) Annotations {
	newMd := m.clone()

	for k, v := range newVs {
		if v.isEmptyValue() {
			delete(newMd, k)
		} else {
			if newMd == nil {
				newMd = make(Annotations)
			}
			newMd[k] = v
		}
	}

	if len(newMd) == 0 {
		return EmptyAnnotations()
	}
	return newMd
}

// clone produces a key-wise copy of the underlying annotations
// publically MD can be copied by reference as it's (by contract) immutable
func (m Annotations) clone() Annotations {

	if m == nil {
		return nil
	}
	newMd := make(Annotations, len(m))
	for ok, ov := range m {
		newMd[ok] = ov
	}
	return newMd
}

// Value implements sql.Valuer, returning a string
func (m Annotations) Value() (driver.Value, error) {
	if len(m) < 1 {
		return driver.Value(string("")), nil
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(m)
	return driver.Value(b.String()), err
}

// Scan implements sql.Scanner
func (m *Annotations) Scan(value interface{}) error {
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
	return fmt.Errorf("annotations invalid db format: %T %T value, err: %v", value, bv, err)
}
