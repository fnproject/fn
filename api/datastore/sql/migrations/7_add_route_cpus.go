package migrations

import (
	"database/sql"
	"github.com/pressly/goose"
)

// we skip the error here to make previous datastore tables work fine
func up7(tx *sql.Tx) error {
	_, _ = tx.Exec("ALTER TABLE routes ADD cpus int;")
	return nil
}

func down7(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE routes DROP COLUMN cpus;")
	return err
}

func init() {
	migrations = append(migrations, &goose.Migration{
		Version:    int64(7),
		UpFn:       up7,
		DownFn:     down7,
		Registered: true,
		Source:     "7_add_route_cpus.go",
	})
}
