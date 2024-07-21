package sql_test

import (
	sqldriver "database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/core/testsuite"
	"github.com/hallgren/eventsourcing/eventstore/sql"
	_ "github.com/mattn/go-sqlite3"
)

func TestSuite(t *testing.T) {
	f := func() (core.EventStore, func(), error) {
		return eventstore()
	}
	testsuite.Test(t, f)
}

func TestMultipleMigrate(t *testing.T) {
	es, close, err := eventstore()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	err = es.Migrate()
	if err != nil {
		t.Fatal(err)
	}
}

func eventstore() (*sql.SQL, func(), error) {
	db, err := sqldriver.Open("sqlite3", "file::memory:?locked.sqlite?cache=shared")
	if err != nil {
		return nil, nil, errors.New(fmt.Sprintf("could not open database %v", err))
	}
	db.SetMaxOpenConns(1)
	err = db.Ping()
	if err != nil {
		return nil, nil, errors.New(fmt.Sprintf("could not ping database %v", err))
	}

	es := sql.Open(db)
	err = es.Migrate()
	if err != nil {
		return nil, nil, errors.New(fmt.Sprintf("could not migrate database %v", err))
	}
	return es, func() {
		es.Close()
	}, nil
}
