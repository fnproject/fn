package models

type ErrorBody struct {
	Message   string `json:"message,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// Validate validates this error body
func (m *ErrorBody) Validate() error {
	return nil
}
