package models

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
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
		v2, _ := c2[k1]
		if v1 != v2 {
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
