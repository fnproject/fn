package models

import "github.com/go-openapi/errors"

type AppWrapper struct {
	App *App `json:"app"`
}

func (m *AppWrapper) Validate() error {
	var res []error

	if err := m.validateApp(); err != nil {
		// prop
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *AppWrapper) validateApp() error {

	if m.App != nil {
		if err := m.App.Validate(); err != nil {
			return err
		}
	}

	return nil
}
