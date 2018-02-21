package migrations

import (
	"database/sql"
	"github.com/pressly/goose"
)

func up4(tx *sql.Tx) error {
	_, _ = tx.Exec("ALTER TABLE routes ADD updated_at VARCHAR(256);")
	return nil
}

func down4(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE routes DROP COLUMN updated_at;")
	return err
}

func init() {
	migrations = append(migrations, &goose.Migration{
		Version:    int64(4),
		UpFn:       up4,
		DownFn:     down4,
		Registered: true,
		Source:     "4_add_route_updated_at.go",
	})
}
