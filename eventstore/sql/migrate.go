package sql

import (
	"context"
)

const createTable = `create table events (seq INTEGER PRIMARY KEY AUTOINCREMENT, id VARCHAR NOT NULL, version INTEGER, reason VARCHAR, type VARCHAR, timestamp VARCHAR, data BLOB, metadata BLOB);`

// Migrate the database
func (s *SQL) Migrate() error {
	sqlStmt := []string{
		createTable,
		`create unique index id_type_version on events (id, type, version);`,
		`create index id_type on events (id, type);`,
	}
	return s.migrate(sqlStmt)
}

func (s *SQL) migrate(stm []string) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// check if the migration is already done
	rows, err := tx.Query(`Select count(*) from events`)
	if err == nil {
		rows.Close()
		return nil
	}

	for _, b := range stm {
		_, err := tx.Exec(b)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
