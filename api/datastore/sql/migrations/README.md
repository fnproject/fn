# Migrations How-To

All migration files should be of the format:

`[0-9]+_[add|remove]_model[_field]*.sql`

The number at the beginning of the file name should be monotonically
increasing, from the last highest file number in this directory. E.g. if there
is `11_add_foo_bar.sql`, your new file should be `12_add_bar_baz.sql`.

Each migration file have to contain both up and down function:
```go
package migrations

import (
	"database/sql"
	"github.com/pressly/goose"
)


func up1(tx *sql.Tx) error {
	_, err := tx.Exec("...")
	return err
}

func down1(tx *sql.Tx) error {
	_, err := tx.Exec("...")
	return err
}

func init() {
	migrations = append(migrations, &goose.Migration{
		Version:    int64(8),
		UpFn:       up1,
		DownFn:     down1,
		Registered: true,
		Source:     "1_add_route_created_at.go",
	})
}

```
Each migration should have initialize `goose.Migration` struct with corresponding version, up/down function, filename (source field).

Please note that every database change should be considered as the migration (new table, new column, column type change, etc.)
