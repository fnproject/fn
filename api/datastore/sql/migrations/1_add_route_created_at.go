package migrations

import (
	"database/sql"
	"github.com/pressly/goose"
)

// we skip the error here to make previous datastore tables work fine
func up1(tx *sql.Tx) error {
	_, _ = tx.Exec("ALTER TABLE routes ADD created_at text;")
	return nil
}

func down1(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE routes DROP COLUMN created_at;")
	return err
}

func init() {
	migrations = append(migrations, &goose.Migration{
		Version:    int64(1),
		UpFn:       up1,
		DownFn:     down1,
		Registered: true,
		Source:     "1_add_route_created_at.go",
	})
}