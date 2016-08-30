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
