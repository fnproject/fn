package route

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fnproject/fn/examples/blog/database"
	"github.com/fnproject/fn/examples/blog/models"
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
