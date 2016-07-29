package models

import "github.com/go-openapi/errors"

type RouteWrapper struct {
	Route *Route `json:"route"`
}

func (m *RouteWrapper) Validate() error {
	var res []error

	if err := m.validateRoute(); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *RouteWrapper) validateRoute() error {

	if m.Route != nil {
		if err := m.Route.Validate(); err != nil {
			return err
		}
	}

	return nil
}
