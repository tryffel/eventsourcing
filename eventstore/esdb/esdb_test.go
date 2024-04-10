//go:build manual
// +build manual

// make these tests manual as they are dependent on a running event store db.

package esdb_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/core/testsuite"
	es "github.com/hallgren/eventsourcing/eventstore/esdb"
)

func TestSuite(t *testing.T) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "eventstore/eventstore:latest",
		ExposedPorts: []string{"2113/tcp", "1113/tcp"},
		WaitingFor:   wait.ForListeningPort("2113/tcp"),
		Cmd:          []string{"--insecure", "--run-projections=All", "--enable-atom-pub-over-http", "--mem-db"},
	}

	eventstoreContainer, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	defer eventstoreContainer.Terminate(ctx)

	endpoint, err := eventstoreContainer.Endpoint(ctx, "esdb")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(endpoint)

	f := func() (core.EventStore, func(), error) {
		// region createClient
		settings, err := esdb.ParseConnectionString(endpoint + "?tls=false")
		if err != nil {
			return nil, nil, err
		}

		db, err := esdb.NewClient(settings)
		if err != nil {
			return nil, nil, err
		}

		es := es.Open(db, true)
		return es, func() {
		}, nil
	}
	testsuite.Test(t, f)
}
