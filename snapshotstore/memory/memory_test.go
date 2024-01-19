package memory_test

import (
	"testing"

	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/core/testsuite"
	"github.com/hallgren/eventsourcing/snapshotstore/memory"
)

func TestSuite(t *testing.T) {
	f := func() (core.SnapshotStore, func(), error) {
		ss := memory.Create()
		return ss, func() { ss.Close() }, nil
	}
	testsuite.TestSnapshotStore(t, f)
}
