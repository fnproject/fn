package migrations

//go:generate go-bindata -ignore migrations.go -ignore index.go -o migrations.go -pkg migrations .

// migrations are generated from this cwd with go generate.
// install https://github.com/jteeuwen/go-bindata for go generate
// command to work properly.

// this will generate a go file with go-bindata of all the migration
// files in 1 go file, so that migrations can be run remotely without
// having to carry the migration files around (i.e. since they are
// compiled into the go binary)
