package server

import (
	"context"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func testRouterAsync(ds models.Datastore, mq models.MessageQueue, rnr agent.Agent) *gin.Engine {
	ctx := context.Background()
	engine := gin.New()
	s := &Server{
		agent:        rnr,
		Router:       engine,
		AdminRouter:  engine,
		datastore:    ds,
		lbReadAccess: ds,
		lbEnqueue:    agent.NewDirectEnqueueAccess(mq),
		mq:           mq,
		nodeType:     ServerTypeFull,
	}

	r := s.Router
	r.Use(gin.Logger())

	s.Router.Use(loggerWrap)
	s.bindHandlers(ctx)
	return r
}
