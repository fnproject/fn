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

package route

import (
	"os"

	"github.com/iron-io/functions/examples/blog/database"
)

func HandlePostRead(db *database.Database, auth map[string]interface{}) {
	id := os.Getenv("PARAM_ID")

	if id == "" {
		SendError("Missing post ID")
		return
	}

	post, err := db.GetPost(id)
	if err != nil {
		SendError("Couldn't retrieve that post")
		return
	}

	SendResponse(Response{
		"post": post,
	})
}
