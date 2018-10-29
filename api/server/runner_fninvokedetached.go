package server

import (
	"net/http"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

// ServeFnInvokeDetached executes the function but it returns an ack as soon as the function starts
func (s *Server) ServeFnInvokeDetached(c *gin.Context, app *models.App, fn *models.Fn) error {
	return s.fnInvokeDetached(c.Writer, c.Request, app, fn)
}

func (s *Server) fnInvokeDetached(resp http.ResponseWriter, req *http.Request, app *models.App, fn *models.Fn) error {

	rw := agent.NewDetachedResponseWriter(resp.Header(), 202)
	opts := []agent.CallOpt{
		agent.WithWriter(rw), // XXX (reed): order matters [for now]
		agent.FromHTTPFnRequest(app, fn, req),
		agent.InvokeDetached(),
	}

	call, err := s.agent.GetCall(opts...)
	if err != nil {
		return err
	}

	err = s.agent.Submit(call)
	if err != nil {
		return err
	}

	if rw.Status > 0 {
		resp.WriteHeader(rw.Status)
	}

	return nil
}
