package migrations

import (
	"database/sql"
	"github.com/pressly/goose"
)

func up6(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE apps ADD updated_at VARCHAR(256);")
	return err
}

func down6(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE apps DROP COLUMN updated_at;")
	return err
}

func init() {
	migrations = append(migrations, &goose.Migration{
		Version:    int64(6),
		UpFn:       up6,
		DownFn:     down6,
		Registered: true,
		Source:     "6_add_app_updated_at.go",
	})
}
