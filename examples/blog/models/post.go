package models

import "gopkg.in/mgo.v2/bson"

type Post struct {
	ID    bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Title string        `json:"title" bson:"title"`
	Body  string        `json:"body" bson:"body"`
	User  string        `json:"user" bsom:"user"`
}
