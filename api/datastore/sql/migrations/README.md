# Migrations How-To

All migration files should be of the format:

`[0-9]+_[add|remove]_model[_field]*.go`

The number at the beginning of the file name should be monotonically
increasing, from the last highest file number in this directory. E.g. if there
is `11_add_foo_bar.go`, your new file should be `12_add_bar_baz.go`.

Each migration file have to contain both up and down function:

```go
package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up1(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE routes ADD created_at text;")
	return err
}

func down1(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE routes DROP COLUMN created_at;")
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(1),
		UpFunc:      up1,
		DownFunc:    down1,
	})
}
```

Each migration must initialize a `migratex.Migration` with corresponding
version and up/down function.

We have elected to expose fn's specific sql migrations as an exported global
list `migrations.Migrations` from this package, you must simply add your
migration and append it to this list.

Please note that every database change should be considered as 1 individual
migration (new table, new column, column type change, etc.)
