package models

type AppWrapper struct {
	App *App `json:"app"`
}

func (m *AppWrapper) Validate() error { return m.validateApp() }

func (m *AppWrapper) validateApp() error {
	if m.App != nil {
		if err := m.App.Validate(); err != nil {
			return err
		}
	}

	return nil
}
