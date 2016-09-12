package models

import "golang.org/x/crypto/bcrypt"

type User struct {
	Username    string `json:"username" bson:"_id,omitempty"`
	Password    []byte `json:"-" bson:"password"`
	NewPassword string `json:"password" bson:"-"`
}

func UserPasswordEncrypt(pass []byte) []byte {
	hashedPassword, err := bcrypt.GenerateFromPassword(pass, bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hashedPassword
}
