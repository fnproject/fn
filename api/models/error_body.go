package models

type ErrorBody struct {
	Message string `json:"message,omitempty"`
	Fields  string `json:"fields,omitempty"`
}

// Validate validates this error body
func (m *ErrorBody) Validate() error {
	return nil
}
