package migrations

import (
	"database/sql"
	"github.com/pressly/goose"
)

// we skip the error here to make previous datastore tables work fine
func up6(tx *sql.Tx) error {
	_, _ = tx.Exec("ALTER TABLE apps ADD updated_at VARCHAR(256);")
	return nil
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
