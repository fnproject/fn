package route

import (
	"encoding/json"
	"fmt"
	"os"

	"gitlab.oracledx.com/odx/functions/examples/blog/database"
	"gitlab.oracledx.com/odx/functions/examples/blog/models"
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
