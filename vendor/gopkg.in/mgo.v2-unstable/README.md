THIS IS UNMAINTAINED
--------------------

Seven years after creating the mgo driver, I'm formally pausing my work on its maintenance.
There are multiple reasons for that, but the main ones are that I've stopped using MongoDB
for any new projects, and supporting its good community was taking too much of my
personal time without a relevant benefit for those around me.

Moving forward I would suggest you to look at one of these options:

  * [globalsign/mgo](https://github.com/globalsign/mgo) - Community supported fork of mgo.
  * [BoltDB](https://github.com/coreos/bbolt) - Single file in-memory document database for Go.
  * [Badger](https://github.com/dgraph-io/badger) - Fast in-memory document database for Go.
  * [DGraph](https://github.com/dgraph-io/dgraph) - Distributed graph database on top of Badger.
  * [lib/pq](https://github.com/lib/pq) - PostgreSQL driver in pure Go.

For technical questions related to mgo, [Stack Overflow](https://stackoverflow.com/questions/tagged/mgo)
is the best place to continue obtaining support from the community.

For personal contact, gustavo at http://niemeyer.net.
