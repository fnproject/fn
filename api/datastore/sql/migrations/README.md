# Migrations How-To

All migration files should be of the format:

`[0-9]+_[add|remove]_model[_field]*.[up|down].sql`

The number at the beginning of the file name should be monotonically
increasing, from the last highest file number in this directory. E.g. if there
is `11_add_foo_bar.up.sql`, your new file should be `12_add_bar_baz.up.sql`.

All `*.up.sql` files must have an accompanying `*.down.sql` file in order to
pass review.

The contents of each file should contain only 1 ANSI sql query. For help, you
may refer to https://github.com/mattes/migrate/blob/master/MIGRATIONS.md which
illustrates some of the finer points.

After creating the file you will need to run, in the same directory as this
README:

```sh
$ go generate
```

NOTE: You may need to `go get github.com/jteeuwen/go-bindata` before running `go
generate` in order for it to work.

After running `go generate`, the `migrations.go` file should be updated. Check
the updated version of this as well as the new `.sql` file into git.

After adding the migration, be sure to update the fields in the sql tables in
`sql.go` up one package. For example, if you added a column `foo` to `routes`,
add this field to the routes `CREATE TABLE` query, as well as any queries
where it should be returned.

After doing this, run the test suite to make sure the sql queries work as
intended and voila. The test suite will ensure that the up and down migrations
work as well as a fresh db. The down migrations will not be tested against
SQLite3 as it does not support `ALTER TABLE DROP COLUMN`, but will still be
tested against postgres and MySQL.
