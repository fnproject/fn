package fnext

import (
	"github.com/fnproject/fn/api/models"
)

// CallOverrider is an interceptor in GetCall which can modify Call and extensions
type CallOverrider func(*models.Call, map[string]string) (map[string]string, error)
