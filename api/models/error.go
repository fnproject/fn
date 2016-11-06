package models

import "errors"

type Error struct {
	Error *ErrorBody `json:"error,omitempty"`
}

func (m *Error) Validate() error {
	return nil
}

var (
	ErrInvalidJSON = errors.New("Invalid JSON")
)
