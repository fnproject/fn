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
