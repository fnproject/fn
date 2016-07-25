package models

type ErrorBody struct {
	Fields  string `json:"fields,omitempty"`
	Message string `json:"message,omitempty"`
}

// Validate validates this error body
func (m *ErrorBody) Validate() error {
	return nil
}
