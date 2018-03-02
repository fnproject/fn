package migrations

import (
	"database/sql"
	"github.com/pressly/goose"
)

func up3(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE calls ADD error text;")
	return err
}

func down3(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE calls DROP COLUMN error;")
	return err
}

func init() {
	migrations = append(migrations, &goose.Migration{
		Version:    int64(3),
		UpFn:       up3,
		DownFn:     down3,
		Registered: true,
		Source:     "3_add_call_error.go",
	})
}
