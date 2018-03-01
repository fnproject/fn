package migrations

import (
	"database/sql"
	"github.com/pressly/goose"
)

// we skip the error here to make previous datastore tables work fine
func up2(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE calls ADD stats text;")
	return err
}

func down2(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE calls DROP COLUMN stats;")
	return err
}

func init() {
	migrations = append(migrations, &goose.Migration{
		Version:    int64(2),
		UpFn:       up2,
		DownFn:     down2,
		Registered: true,
		Source:     "2_add_call_stats.go",
	})
}
