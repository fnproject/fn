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

package ifaces

import (
	"net/http"

	"github.com/iron-io/functions/api/models"
)

type SpecialHandler interface {
	Handle(c HandlerContext) error
}

// Each handler can modify the context here so when it gets passed along, it will use the new info.
// Not using Gin's Context so we don't lock ourselves into Gin, this is a subset of the Gin context.
type HandlerContext interface {
	// Request returns the underlying http.Request object
	Request() *http.Request

	// Datastore returns the models.Datastore object. Not that this has arbitrary key value store methods that can be used to store extra data
	Datastore() models.Datastore

	// Set and Get values on the context, this can be useful to change behavior for the rest of the request
	Set(key string, value interface{})
	Get(key string) (value interface{}, exists bool)
}
