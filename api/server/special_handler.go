package server

import (
	"context"
	"errors"
	"net/http"
)

var ErrNoSpecialHandlerFound = errors.New("Path not found")

type SpecialHandler interface {
	Handle(c HandlerContext) error
}

// Each handler can modify the context here so when it gets passed along, it will use the new info.
type HandlerContext interface {
	// Context return the context object
	Context() context.Context

	// Request returns the underlying http.Request object
	Request() *http.Request

	// Response returns the http.ResponseWriter
	Response() http.ResponseWriter

	// Overwrite value in the context
	Set(key string, value interface{})
}

type SpecialHandlerContext struct {
	request  *http.Request
	response http.ResponseWriter
	ctx      context.Context
}

func (c *SpecialHandlerContext) Context() context.Context {
	return c.ctx
}

func (c *SpecialHandlerContext) Request() *http.Request {
	return c.request
}

func (c *SpecialHandlerContext) Response() http.ResponseWriter {
	return c.response
}

func (c *SpecialHandlerContext) Set(key string, value interface{}) {
	c.ctx = context.WithValue(c.ctx, key, value)
}

func (s *Server) AddSpecialHandler(handler SpecialHandler) {
	s.specialHandlers = append(s.specialHandlers, handler)
}

// UseSpecialHandlers execute all special handlers
func (s *Server) UseSpecialHandlers(ctx context.Context, req *http.Request, resp http.ResponseWriter) (context.Context, error) {
	if len(s.specialHandlers) == 0 {
		return ctx, ErrNoSpecialHandlerFound
	}

	c := &SpecialHandlerContext{
		request:  req,
		response: resp,
		ctx:      ctx,
	}
	for _, l := range s.specialHandlers {
		err := l.Handle(c)
		if err != nil {
			return c.ctx, err
		}
	}
	return c.ctx, nil
}
