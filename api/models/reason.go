package models

import (
	"encoding/json"

	strfmt "github.com/go-openapi/strfmt"

	"github.com/go-openapi/validate"
)

/*Reason Machine usable reason for job being in this state.
Valid values for error status are `timeout | killed | bad_exit`.
Valid values for cancelled status are `client_request`.
For everything else, this is undefined.


swagger:model Reason
*/
type Reason string

// for schema
var reasonEnum []interface{}

func (m Reason) validateReasonEnum(path, location string, value Reason) error {
	if reasonEnum == nil {
		var res []Reason
		if err := json.Unmarshal([]byte(`["timeout","killed","bad_exit","client_request"]`), &res); err != nil {
			return err
		}
		for _, v := range res {
			reasonEnum = append(reasonEnum, v)
		}
	}
	err := validate.Enum(path, location, value, reasonEnum)
	return err
}

// Validate validates this reason
func (m Reason) Validate(formats strfmt.Registry) error {
	// value enum
	return m.validateReasonEnum("", "body", m)
}
