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
