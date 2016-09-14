package database

import "gopkg.in/mgo.v2"

type Database struct {
	Session *mgo.Session
}

func New(uri string) *Database {
	session, err := mgo.Dial(uri)
	if err != nil {
		panic(err)
	}

	session.SetMode(mgo.Monotonic, true)

	return &Database{
		Session: session,
	}
}
