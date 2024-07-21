package sql

import "context"

const createTable = `create table snapshots (id VARCHAR NOT NULL, type VARCHAR, version INTEGER, global_version INTEGER, state BLOB);`

// Migrate the database
func (s *SQL) Migrate() error {
	sqlStmt := []string{
		createTable,
		`create unique index id_type on snapshots (id, type);`,
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
	rows, err := tx.Query(`Select count(*) from snapshots`)
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
