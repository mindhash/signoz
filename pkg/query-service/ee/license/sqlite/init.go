package sqlite

import (
	"fmt"
	"github.com/jmoiron/sqlx"
)

func InitDB(db *sqlx.DB) error {
	var err error
	if db == nil {
		return fmt.Errorf("invalid db connection")
	}

	table_schema := `CREATE TABLE IF NOT EXISTS licenses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		uuid TEXT NOT NULL UNIQUE,
		created_at datetime NOT NULL, 
		key TEXT NOT NULL,
		start datetime NULL,
		end datetime NULL,
		org_id TEXT NOT NULL
	);`

	_, err = db.Exec(table_schema)
	if err != nil {
		return fmt.Errorf("Error in creating licenses table: %s", err.Error())
	}
	return nil
}
