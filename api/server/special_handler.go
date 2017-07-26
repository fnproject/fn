package server

import (
	"net/http"

	"github.com/fnproject/fn/api/models"
)

// SpecialHandler verysimilar to a handler but since it is not used as middle ware no way
// to get context without returning it. So we just return a request which could have newly made
// contexts.
type SpecialHandler interface {
	Handle(w http.ResponseWriter, r *http.Request) (*http.Request, error)
}

// AddSpecialHandler adds the SpecialHandler to the specialHandlers list.
func (s *Server) AddSpecialHandler(handler SpecialHandler) {
	s.specialHandlers = append(s.specialHandlers, handler)
}

// UseSpecialHandlers execute all special handlers
func (s *Server) UseSpecialHandlers(resp http.ResponseWriter, req *http.Request) (*http.Request, error) {
	if len(s.specialHandlers) == 0 {
		return req, models.ErrNoSpecialHandlerFound
	}
	var r *http.Request
	var err error

	for _, l := range s.specialHandlers {
		r, err = l.Handle(resp, req)
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}
