package database

import (
	"github.com/iron-io/functions/examples/blog/models"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func (db *Database) SaveUser(user *models.User) (*models.User, error) {
	s := db.Session.Copy()
	defer s.Close()

	c := s.DB("").C("user")
	id := user.Username
	user.Username = ""

	if len(user.Password) > 0 {
		user.Password = models.UserPasswordEncrypt(user.Password)
	}

	change := mgo.Change{
		Update:    bson.M{"$set": user},
		ReturnNew: true,
		Upsert:    true,
	}

	_, err := c.Find(bson.M{"_id": id}).Apply(change, &user)

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (db *Database) GetUser(id string) (*models.User, error) {
	s := db.Session.Copy()
	defer s.Close()

	c := s.DB("").C("user")

	var user models.User

	err := c.Find(bson.M{"_id": id}).One(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (db *Database) GetUsers(query []bson.M) ([]*models.User, error) {
	s := db.Session.Copy()
	defer s.Close()

	c := s.DB("").C("user")

	var users []*models.User

	err := c.Pipe(query).All(&users)
	if err != nil {
		return nil, err
	}
	return users, nil
}
