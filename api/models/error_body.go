package models

type Error struct {
	Message string `json:"message,omitempty"`
	Fields  string `json:"fields,omitempty"`
}

// Validate validates this error body
func (m *Error) Validate() error {
	return nil
}
