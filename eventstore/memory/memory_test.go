package memory_test

import (
	"testing"

	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/core/suite"
	"github.com/hallgren/eventsourcing/eventstore/memory"
)

func TestSuite(t *testing.T) {
	f := func() (core.EventStore, func(), error) {
		es := memory.Create()
		return es, func() { es.Close() }, nil
	}
	suite.Test(t, f)
}
