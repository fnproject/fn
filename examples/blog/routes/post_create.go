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
	"encoding/json"
	"fmt"
	"os"

	"github.com/iron-io/functions/examples/blog/database"
	"github.com/iron-io/functions/examples/blog/models"
)

func HandlePostCreate(db *database.Database, auth map[string]interface{}) {
	var post *models.Post

	if err := json.NewDecoder(os.Stdin).Decode(&post); err != nil {
		fmt.Printf("Couldn't decode post JSON: %v\n", err)
		return
	}

	post, err := db.SavePost(post)
	if err != nil {
		fmt.Println("Couldn't save that post")
		return
	}

	post.User = auth["user"].(string)

	SendResponse(Response{
		"post": post,
	})
}
