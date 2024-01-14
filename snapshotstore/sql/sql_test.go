package sql_test

import (
	"context"
	sqldriver "database/sql"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/snapshotstore/sql"
	_ "github.com/proullon/ramsql/driver"
)

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func TestSaveAndGet(t *testing.T) {
	r := seededRand.Intn(999999999999)
	db, err := sqldriver.Open("ramsql", fmt.Sprintf("%d", r))
	if err != nil {
		t.Fatalf("could not open ramsql database %v", err)
	}
	store := sql.Open(db)
	store.MigrateTest()

	snapshot := core.Snapshot{
		ID:            "id",
		Type:          "person",
		Version:       1,
		GlobalVersion: 1,
		State:         []byte("123"),
	}

	err = store.Save(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	// update snapshot
	snapshot.State = []byte("456")
	snapshot.Version = 2

	err = store.Save(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	snapshot2, err := store.Get(context.Background(), "id", "person")
	if err != nil {
		t.Fatal(err)
	}

	if string(snapshot.State) != string(snapshot2.State) {
		t.Fatal("wrong state")
	}
}

func TestGetNoneExistingSnapshot(t *testing.T) {
	r := seededRand.Intn(999999999999)
	db, err := sqldriver.Open("ramsql", fmt.Sprintf("%d", r))
	if err != nil {
		t.Fatalf("could not open ramsql database %v", err)
	}
	store := sql.Open(db)
	store.MigrateTest()

	_, err = store.Get(context.Background(), "id", "person")
	if !errors.Is(err, core.ErrSnapshotNotFound) {
		t.Fatal(err)
	}
}
