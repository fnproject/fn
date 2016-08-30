package route

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/iron-io/functions/examples/blog/database"
	"github.com/iron-io/functions/examples/blog/models"
	"golang.org/x/crypto/bcrypt"
)

// import "github.com/iron-io/functions/examples/blog/database"

var jwtSignKey = []byte("mysecretblog")

type Response map[string]interface{}

func SendResponse(resp Response) {
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func SendError(err interface{}) {
	SendResponse(Response{
		"error": err,
	})
}

func HandleToken(db *database.Database) {
	var login *models.User
	err := json.Unmarshal([]byte(os.Getenv("PAYLOAD")), &login)
	if err != nil {
		fmt.Println("Missing username and password")
		return
	}

	user, err := db.GetUser(login.Username)
	if err != nil {
		SendError("Couldn't create a token")
		return
	}

	if err := bcrypt.CompareHashAndPassword(user.Password, []byte(login.NewPassword)); err != nil {
		SendError("Couldn't create a token")
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user": login.Username,
		"exp":  time.Now().Add(1 * time.Hour),
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString(jwtSignKey)
	if err != nil {
		SendError("Couldn't create a token")
		return
	}

	SendResponse(Response{"token": tokenString})
}

func Authentication() (map[string]interface{}, bool) {
	authorization := os.Getenv("HEADER_AUTHORIZATION")

	p := strings.Split(authorization, " ")
	if len(p) <= 1 {
		return nil, false
	}

	token, err := jwt.Parse(p[1], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSignKey, nil
	})

	if err != nil {
		return nil, false
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, true
	}

	return nil, false
}

// func New(db *database.Database) *gin.Engine {
// 	DB = db

// 	r := gin.New()
// 	r.POST("/auth", func(c *gin.Context) {
// 		username := c.PostForm("username")
// 		password := c.PostForm("password")

// 		user, err := db.GetUser(username)
// 		if err != nil {
// 			c.JSON(500, gin.H{"message": "Could not generate token"})
// 			return
// 		}

// 		err = bcrypt.CompareHashAndPassword(user.Password, []byte(password))
// 		if err != nil {
// 			c.JSON(500, gin.H{"message": "Could not generate token"})
// 			return
// 		}

// 		token := jwt_lib.New(jwt_lib.GetSigningMethod("HS256"))
// 		claims := token.Claims.(jwt_lib.MapClaims)
// 		claims["ID"] = username
// 		claims["exp"] = time.Now().Add(time.Hour * 1).Unix()

// 		tokenString, err := token.SignedString([]byte(jwtSignKey))
// 		if err != nil {
// 			c.JSON(500, gin.H{"message": "Could not generate token"})
// 			return
// 		}
// 		c.JSON(200, gin.H{"token": tokenString})
// 	})

// 	r.POST("/testuser", func(c *gin.Context) {
// 		_, err := db.SaveUser(&models.User{
// 			Username: "test",
// 			Password: []byte("test"),
// 		})
// 		if err != nil {
// 			c.JSON(500, gin.H{"message": "Could create test user"})
// 			return
// 		}
// 		c.JSON(200, gin.H{"message": "test user created"})
// 	})

// 	blog := r.Group("/blog")
// 	blog.Use(jwtAuth(jwtSignKey))
// 	blog.GET("/posts", handlePostList)
// 	blog.POST("/posts", handlePostCreate)
// 	blog.GET("/posts/:id", handlePostRead)

// 	return r
// }
