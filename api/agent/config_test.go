package agent

import (
	"errors"
	"os"
	"testing"
)

// TestSetEnvUintPointer tests the normal use cases
func TestSetEnvUintPointer(t *testing.T) {
	valid := uint64(1111)
	defaultValue := uint64(2222)

	os.Setenv("FN_TEST_VALID", "1111")

	tests := []struct {
		Name     string
		EnvVar   string
		Default  *uint64
		Expected *uint64
	}{
		{"EnvVarNoDefault", "FN_TEST_VALID", nil, &valid},
		{"EnvVarDefault", "FN_TEST_VALID", &defaultValue, &valid},
		{"NoEnvVarNoDefault", "FN_TEST_NON_EXISTENT", nil, nil},
		{"NoEnvVarDefault", "FN_TEST_NON_EXISTENT", &defaultValue, &defaultValue},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var val *uint64
			outErr := setEnvUintPointer(nil, tt.EnvVar, &val, tt.Default)
			if outErr != nil {
				t.Fatal("Unexpected error returned from setEnvUintPointer")
			}

			if tt.Expected == nil && val == nil {
				return
			}

			if tt.Expected == nil {
				t.Fatalf("expected a nil value; got %d", *val)
			}

			if val == nil {
				t.Fatalf("expected %d; got a nil value", *tt.Expected)
			}

			if *tt.Expected != *val {
				t.Fatalf("expected %d; got %d", *tt.Expected, *val)
			}
		})
	}
}

// TestSetEnvUintPointerError tests the error use cases
func TestSetEnvUintPointerError(t *testing.T) {
	defaultValue := uint64(2222)

	os.Setenv("FN_TEST_VALID", "1111")
	os.Setenv("FN_TEST_INVALID", "not a valid uint64")

	tests := []struct {
		Name   string
		EnvVar string
		Error  error
	}{
		{"ErrorInput", "FN_TEST_VALID", errors.New("error")},
		{"InvalidEnvVar", "FN_TEST_INVALID", nil},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var val *uint64
			outErr := setEnvUintPointer(tt.Error, tt.EnvVar, &val, &defaultValue)
			if outErr == nil {
				t.Fatal("Expecting a error from setEnvUintPointer")
			}
		})
	}
}

// TestSetEnvUint tests the normal use cases
func TestSetEnvUint(t *testing.T) {
	valid := uint64(1111)
	defaultValue := uint64(2222)

	os.Setenv("FN_TEST_VALID", "1111")

	tests := []struct {
		Name     string
		EnvVar   string
		Default  *uint64
		Expected uint64
	}{
		{"EnvVarNoDefault", "FN_TEST_VALID", nil, valid},
		{"EnvVarDefault", "FN_TEST_VALID", &defaultValue, valid},
		{"NoEnvVarNoDefault", "FN_TEST_NON_EXISTENT", nil, 0},
		{"NoEnvVarDefault", "FN_TEST_NON_EXISTENT", &defaultValue, defaultValue},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var val uint64
			outErr := setEnvUint(nil, tt.EnvVar, &val, tt.Default)
			if outErr != nil {
				t.Fatal("Unexpected error returned from setEnvUint")
			}

			if tt.Expected != val {
				t.Fatalf("expected %d; got %d", tt.Expected, val)
			}
		})
	}
}

// TestSetEnvUintError tests the error use cases
func TestSetEnvUintError(t *testing.T) {
	defaultValue := uint64(2222)

	os.Setenv("FN_TEST_VALID", "1111")
	os.Setenv("FN_TEST_INVALID", "not a valid uint64")

	tests := []struct {
		Name   string
		EnvVar string
		Error  error
	}{
		{"ErrorInput", "FN_TEST_VALID", errors.New("error")},
		{"InvalidEnvVar", "FN_TEST_INVALID", nil},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var val uint64
			outErr := setEnvUint(tt.Error, tt.EnvVar, &val, &defaultValue)
			if outErr == nil {
				t.Fatal("Expecting a error from setEnvUint")
			}
		})
	}
}
