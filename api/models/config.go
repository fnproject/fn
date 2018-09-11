package models

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type Config map[string]string

func (c *Config) Validate() error {
	return nil
}

func (c1 Config) Equals(c2 Config) bool {
	if len(c1) != len(c2) {
		return false
	}
	for k1, v1 := range c1 {
		if v2, ok := c2[k1]; !ok || v1 != v2 {
			return false
		}
	}
	return true
}

// implements sql.Valuer, returning a string
func (c Config) Value() (driver.Value, error) {
	if len(c) < 1 {
		return driver.Value(string("")), nil
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(c)
	// return a string type
	return driver.Value(b.String()), err
}

// implements sql.Scanner
func (c *Config) Scan(value interface{}) error {
	if value == nil {
		*c = nil
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
			return json.Unmarshal(b, c)
		}

		*c = nil
		return nil
	}

	// otherwise, return an error
	return fmt.Errorf("config invalid db format: %T %T value, err: %v", value, bv, err)
}

// Headers is an http.Header that implements additional methods.
type Headers http.Header

func (h1 Headers) Equals(h2 Headers) bool {
	if len(h1) != len(h2) {
		return false
	}
	for k1, v1s := range h1 {
		v2s, _ := h2[k1]
		if len(v2s) != len(v1s) {
			return false
		}
		for i, v1 := range v1s {
			if v2s[i] != v1 {
				return false
			}
		}
	}
	return true
}

// implements sql.Valuer, returning a string
func (h Headers) Value() (driver.Value, error) {
	if len(h) < 1 {
		return driver.Value(string("")), nil
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(h)
	// return a string type
	return driver.Value(b.String()), err
}

// implements sql.Scanner
func (h *Headers) Scan(value interface{}) error {
	if value == nil {
		*h = nil
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
			return json.Unmarshal(b, h)
		}

		*h = nil
		return nil
	}

	// otherwise, return an error
	return fmt.Errorf("headers invalid db format: %T %T value, err: %v", value, bv, err)
}

// MilliCPU units
type MilliCPUs uint64

const (
	MinMilliCPUs = 0       // 0 is unlimited
	MaxMilliCPUs = 1024000 // 1024 CPUs
)

// implements fmt.Stringer
func (c MilliCPUs) String() string {
	if c == 0 {
		return ""
	}
	return fmt.Sprintf("%dm", c)
}

// implements json.Unmarshaler
func (c *MilliCPUs) UnmarshalJSON(data []byte) error {

	outer := bytes.TrimSpace(data)

	if bytes.Equal(outer, []byte("null")) {
		*c = MilliCPUs(0)
		return nil
	}

	if !bytes.HasSuffix(outer, []byte("\"")) || !bytes.HasPrefix(outer, []byte("\"")) {
		return ErrInvalidJSON
	}

	outer = bytes.TrimPrefix(outer, []byte("\""))
	outer = bytes.TrimSuffix(outer, []byte("\""))
	outer = bytes.TrimSpace(outer)
	if len(outer) == 0 {
		*c = 0
		return nil
	}

	if bytes.HasSuffix(outer, []byte("m")) {

		// Support milli cores as "100m"
		outer = bytes.TrimSuffix(outer, []byte("m"))
		mCPU, err := strconv.ParseUint(string(outer), 10, 64)
		if err != nil || mCPU > MaxMilliCPUs || mCPU < MinMilliCPUs {
			return ErrInvalidCPUs
		}
		*c = MilliCPUs(mCPU)
	} else {
		// Support for floating point "0.1" style CPU units
		fCPU, err := strconv.ParseFloat(string(outer), 64)
		if err != nil || fCPU < MinMilliCPUs/1000 || fCPU > MaxMilliCPUs/1000 {
			return ErrInvalidCPUs
		}
		*c = MilliCPUs(fCPU * 1000)
	}

	return nil
}

// implements json.Marshaler
func (c *MilliCPUs) MarshalJSON() ([]byte, error) {

	if *c < MinMilliCPUs || *c > MaxMilliCPUs {
		return nil, ErrInvalidCPUs
	}

	// always use milli cpus "1000m" format
	return []byte(fmt.Sprintf("\"%s\"", c.String())), nil
}
