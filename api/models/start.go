package models

import "github.com/go-openapi/strfmt"

/*Start start

swagger:model Start
*/
type Start struct {

	/* Time when task started execution. Always in UTC.
	 */
	StartedAt strfmt.DateTime `json:"started_at,omitempty"`
}

// Validate validates this start
func (m *Start) Validate(formats strfmt.Registry) error {
	return nil
}
