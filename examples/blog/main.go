package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/iron-io/functions/examples/blog/database"
	"github.com/iron-io/functions/examples/blog/models"
	"github.com/iron-io/functions/examples/blog/routes"
)

func main() {
	request := fmt.Sprintf("%s %s", os.Getenv("METHOD"), os.Getenv("ROUTE"))

	dbURI := os.Getenv("CONFIG_DB")
	if dbURI == "" {
		dbURI = "127.0.0.1/blog"
	}
	db := database.New(dbURI)

	if created := createUser(db); created {
		return
	}

	if os.Getenv("ROUTE") == "/token" {
		route.HandleToken(db)
		return
	}

	auth, valid := route.Authentication()
	if !valid {
		route.SendError("Invalid authentication")
		return
	}

	switch request {
	case "GET /posts":
		route.HandlePostList(db, auth)
		break
	case "POST /posts":
		route.HandlePostCreate(db, auth)
		break
	case "GET /posts/:id":
		route.HandlePostRead(db, auth)
		break
	default:
		route.SendError("Not found")
	}
}

func createUser(db *database.Database) bool {
	env := os.Getenv("NEWUSER")

	if env == "" {
		return false
	}

	var user *models.User
	err := json.Unmarshal([]byte(env), &user)
	if err != nil {
		fmt.Println(err)
		return true
	}

	if user.Username == "" || user.NewPassword == "" {
		fmt.Println("missing username or password")
		return true
	}

	user.Password = []byte(user.NewPassword)
	user.NewPassword = ""

	user, err = db.SaveUser(user)
	if err != nil {
		fmt.Println("couldn't create user")
	} else {
		fmt.Println("user created")
	}

	return true
}
