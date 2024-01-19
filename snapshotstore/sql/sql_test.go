package sql_test

import (
	sqldriver "database/sql"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/core/testsuite"
	"github.com/hallgren/eventsourcing/snapshotstore/sql"
	_ "github.com/proullon/ramsql/driver"
)

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func TestSuite(t *testing.T) {
	f := func() (core.SnapshotStore, func(), error) {
		r := seededRand.Intn(999999999999)
		db, err := sqldriver.Open("ramsql", fmt.Sprintf("%d", r))
		if err != nil {
			t.Fatalf("could not open ramsql database %v", err)
		}
		store := sql.Open(db)
		store.MigrateTest()
		return store, func() { store.Close() }, nil
	}
	testsuite.TestSnapshotStore(t, f)
}
