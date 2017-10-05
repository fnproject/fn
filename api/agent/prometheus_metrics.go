package agent

import (
	"net/http"
)

func (a *agent) PromHandler() http.Handler {
	return a.promHandler
}
