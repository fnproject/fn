package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

type SpecialHandlerContext struct {
	server     *Server
	ginContext *gin.Context
}

func (c *SpecialHandlerContext) Request() *http.Request {
	return c.ginContext.Request
}

func (c *SpecialHandlerContext) Datastore() models.Datastore {
	return c.server.Datastore
}

func (c *SpecialHandlerContext) Set(key string, value interface{}) {
	c.ginContext.Set(key, value)
}
func (c *SpecialHandlerContext) Get(key string) (value interface{}, exists bool) {
	return c.ginContext.Get(key)
}
