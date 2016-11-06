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

	if err := json.NewDecoder(os.Stdin).Decode(&login); err != nil {
		fmt.Printf("Couldn't decode login JSON: %v\n", err)
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
