package models

import (
	strfmt "github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"

	"github.com/go-openapi/errors"
)

type AppsWrapper struct {
	Apps []*App `json:"apps"`
}

func (m *AppsWrapper) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateApps(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *AppsWrapper) validateApps(formats strfmt.Registry) error {

	if err := validate.Required("apps", "body", m.Apps); err != nil {
		return err
	}

	return nil
}
