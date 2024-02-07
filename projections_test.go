package eventsourcing_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hallgren/eventsourcing"
	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/eventstore/memory"
	"github.com/hallgren/eventsourcing/internal"
)

func createBornEvent(es *memory.Memory, name string) error {
	person, err := CreatePerson(name)
	if err != nil {
		return err
	}

	events := make([]core.Event, 0)
	for _, e := range person.Events() {
		data, err := json.Marshal(e.Data())
		if err != nil {
			return err
		}

		events = append(events, core.Event{
			AggregateID:   e.AggregateID(),
			Reason:        e.Reason(),
			AggregateType: e.AggregateType(),
			Version:       core.Version(e.Version()),
			GlobalVersion: core.Version(e.GlobalVersion()),
			Timestamp:     e.Timestamp(),
			Data:          data,
		})
	}
	return es.Save(events)
}

func TestRunOnce(t *testing.T) {
	// setup
	es := memory.Create()
	register := internal.NewRegister()
	register.Register(&Person{})

	projectedName := ""

	err := createBornEvent(es, "kalle")
	if err != nil {
		t.Fatal(err)
	}

	err = createBornEvent(es, "anka")
	if err != nil {
		t.Fatal(err)
	}

	// run projection one event at each run
	p := eventsourcing.NewProjections(register, json.Unmarshal)
	proj := p.Add(es.GlobalEvents(0, 1), func(event eventsourcing.Event) error {
		switch e := event.Data().(type) {
		case *Born:
			projectedName = e.Name
		}
		return nil
	}, time.Second)

	// should set projectedName to kalle
	err, work := proj.RunOnce()
	if err != nil {
		t.Fatal(err)
	}

	if !work {
		t.Fatal("there was no work to do")
	}
	if projectedName != "kalle" {
		t.Fatalf("expected %q was %q", "kalle", projectedName)
	}

	// should set the projected name to anka
	err, work = proj.RunOnce()
	if err != nil {
		t.Fatal(err)
	}

	if !work {
		t.Fatal("there was no work to do")
	}
	if projectedName != "anka" {
		t.Fatalf("expected %q was %q", "anka", projectedName)
	}
}

func TestRun(t *testing.T) {
	// setup
	es := memory.Create()
	register := internal.NewRegister()
	register.Register(&Person{})

	projectedName := ""
	sourceName := "kalle"

	err := createBornEvent(es, sourceName)
	if err != nil {
		t.Fatal(err)
	}

	// run projection
	p := eventsourcing.NewProjections(register, json.Unmarshal)
	proj := p.Add(es.GlobalEvents(0, 1), func(event eventsourcing.Event) error {
		switch e := event.Data().(type) {
		case *Born:
			projectedName = e.Name
		}
		return nil
	}, time.Second)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()

	// will run once then sleep 10 seconds
	err = proj.Run(ctx, time.Second*10)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}

	if projectedName != sourceName {
		t.Fatalf("expected %q was %q", sourceName, projectedName)
	}
}

func TestCloseNoneStarted(t *testing.T) {
	p := eventsourcing.NewProjections(nil, json.Unmarshal)
	p.Close()
}

func TestStartMultipleProjections(t *testing.T) {
	// setup
	es := memory.Create()
	register := internal.NewRegister()

	// run projection
	p := eventsourcing.NewProjections(register, json.Unmarshal)
	p.Add(es.GlobalEvents(0, 1), func(event eventsourcing.Event) error { return nil }, time.Second)
	p.Add(es.GlobalEvents(0, 1), func(event eventsourcing.Event) error { return nil }, time.Second)
	p.Add(es.GlobalEvents(0, 1), func(event eventsourcing.Event) error { return nil }, time.Second)

	p.Start()
	p.Close()
}
