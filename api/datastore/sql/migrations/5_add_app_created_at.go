package migrations

import (
	"database/sql"
	"github.com/pressly/goose"
)

// we skip the error here to make previous datastore tables work fine
func up5(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE apps ADD created_at VARCHAR(256);")
	return err
}

func down5(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE apps DROP COLUMN created_at;")
	return err
}

func init() {
	migrations = append(migrations, &goose.Migration{
		Version:    int64(5),
		UpFn:       up5,
		DownFn:     down5,
		Registered: true,
		Source:     "5_add_app_created_at.go",
	})
}
