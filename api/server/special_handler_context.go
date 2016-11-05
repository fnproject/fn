// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
