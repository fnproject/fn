package common

import (
	"context"

	"github.com/fnproject/fn/api/id"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type key string

const (
	ridLabel = "request-id"
	// RidKey is the name of the key saved in the context for holding the request ID
	RidKey    = key(ridLabel)
	maxLenght = 32
)

func RequestIDInCtxAndLogger(headerName string) func(c *gin.Context) {
	return func(c *gin.Context) {
		rid := c.Request.Header.Get(headerName)
		if rid == "" {
			rid = id.New().String()
		}
		// we truncate the rid to the value specified by maxLenght const
		rid = rid[:maxLenght]
		ctx := context.WithValue(c.Request.Context(), RidKey, rid)
		// We set the rid in the common logger so it is always logged when the common logger is used
		l := Logger(ctx).WithFields(logrus.Fields{ridLabel: rid})
		ctx = WithLogger(ctx, l)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
