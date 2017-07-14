package models

type RouteWrapper struct {
	Route *Route `json:"route"`
}

func (m *RouteWrapper) Validate(skipZero bool) error { return m.validateRoute(skipZero) }

func (m *RouteWrapper) validateRoute(skipZero bool) error {

	if m.Route != nil {
		if err := m.Route.Validate(skipZero); err != nil {
			return err
		}
	}

	return nil
}
