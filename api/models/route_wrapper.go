package models

type RouteWrapper struct {
	Route *Route `json:"route"`
}

func (m *RouteWrapper) Validate() error {
	if m.Route != nil {
		return m.Route.Validate()
	}
	return nil
}
