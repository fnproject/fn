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
	"errors"

	"github.com/iron-io/functions/examples/blog/models"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	ErrNotObjectIdHex = errors.New("Invalid ID")
)

func (db *Database) SavePost(post *models.Post) (*models.Post, error) {
	s := db.Session.Copy()
	defer s.Close()

	c := s.DB("").C("post")

	if post.ID.Hex() == "" {
		post.ID = bson.NewObjectId()
	}
	id := post.ID

	change := mgo.Change{
		Update:    bson.M{"$set": post},
		ReturnNew: true,
		Upsert:    true,
	}

	_, err := c.Find(bson.M{"_id": id}).Apply(change, &post)

	if err != nil {
		return nil, err
	}

	return post, nil
}

func (db *Database) GetPost(id string) (*models.Post, error) {
	s := db.Session.Copy()
	defer s.Close()

	c := s.DB("").C("post")

	var post models.Post
	if !bson.IsObjectIdHex(id) {
		return nil, ErrNotObjectIdHex
	}

	err := c.Find(bson.M{"_id": bson.ObjectIdHex(id)}).One(&post)
	if err != nil {
		return nil, err
	}

	return &post, nil
}

func (db *Database) GetPosts(query []bson.M) ([]*models.Post, error) {
	s := db.Session.Copy()
	defer s.Close()

	c := s.DB("").C("post")

	var posts []*models.Post

	err := c.Pipe(query).All(&posts)
	if err != nil {
		return nil, err
	}
	return posts, nil
}
