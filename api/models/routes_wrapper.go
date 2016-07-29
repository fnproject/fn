package models

import (
	strfmt "github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"

	"github.com/go-openapi/errors"
)

type RoutesWrapper struct {
	Cursor string     `json:"cursor,omitempty"`
	Error  *ErrorBody `json:"error,omitempty"`
	Routes []*Route   `json:"routes"`
}

func (m *RoutesWrapper) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateRoutes(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *RoutesWrapper) validateRoutes(formats strfmt.Registry) error {

	if err := validate.Required("routes", "body", m.Routes); err != nil {
		return err
	}

	return nil
}
