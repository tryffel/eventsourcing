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
)

func createPersonEvent(es *memory.Memory, name string, age int) error {
	person, err := CreatePerson(name)
	if err != nil {
		return err
	}

	for i := 0; i < age; i++ {
		person.GrowOlder()
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
	register := eventsourcing.NewRegister()
	register.Register(&Person{})

	projectedName := ""

	err := createPersonEvent(es, "kalle", 0)
	if err != nil {
		t.Fatal(err)
	}

	err = createPersonEvent(es, "anka", 0)
	if err != nil {
		t.Fatal(err)
	}

	// run projection one event at each run
	p := eventsourcing.NewProjections(register, json.Unmarshal)
	proj := p.NewRunner(es.GlobalEvents(0, 1), func(event eventsourcing.Event) error {
		switch e := event.Data().(type) {
		case *Born:
			projectedName = e.Name
		}
		return nil
	})

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
	register := eventsourcing.NewRegister()
	register.Register(&Person{})

	projectedName := ""
	sourceName := "kalle"

	err := createPersonEvent(es, sourceName, 1)
	if err != nil {
		t.Fatal(err)
	}

	// run projection
	p := eventsourcing.NewProjections(register, json.Unmarshal)
	proj := p.NewRunner(es.GlobalEvents(0, 1), func(event eventsourcing.Event) error {
		switch e := event.Data().(type) {
		case *Born:
			projectedName = e.Name
		}
		return nil
	})

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()

	// will run once then sleep 10 seconds
	err = proj.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}

	if projectedName != sourceName {
		t.Fatalf("expected %q was %q", sourceName, projectedName)
	}
}

func TestCloseNoneStartedProjection(t *testing.T) {
	p := eventsourcing.NewProjections(nil, json.Unmarshal)
	p.Close()
}

func TestStartMultipleProjections(t *testing.T) {
	// setup
	es := memory.Create()
	register := eventsourcing.NewRegister()

	// callback that handles the events
	callbackF := func(event eventsourcing.Event) error {
		return nil
	}

	// run projection
	p := eventsourcing.NewProjections(register, json.Unmarshal)
	p.NewRunner(es.GlobalEvents(0, 1), callbackF)
	p.NewRunner(es.GlobalEvents(0, 1), callbackF)
	p.NewRunner(es.GlobalEvents(0, 1), callbackF)

	p.Start()
	p.Close()
}

func TestErrorFromCallback(t *testing.T) {
	// setup
	es := memory.Create()
	register := eventsourcing.NewRegister()
	register.Register(&Person{})

	err := createPersonEvent(es, "kalle", 1)
	if err != nil {
		t.Fatal(err)
	}

	// define application error that can be returned from the callback function
	var ErrApplication = errors.New("application error")

	// callback that handles the events
	callbackF := func(event eventsourcing.Event) error {
		return ErrApplication
	}

	// run projection
	p := eventsourcing.NewProjections(register, json.Unmarshal)
	p.NewRunner(es.GlobalEvents(0, 1), callbackF)

	errChan := p.Start()
	defer p.Close()

	err = <-errChan
	if !errors.Is(err, ErrApplication) {
		if err != nil {
			t.Fatalf("expected application error but got %s", err.Error())
		}
		t.Fatal("got none error expected ErrApplication")
	}
}

func TestStrict(t *testing.T) {
	// setup
	es := memory.Create()
	register := eventsourcing.NewRegister()

	// We do not register the Person aggregate with the Born event attached
	err := createPersonEvent(es, "kalle", 1)
	if err != nil {
		t.Fatal(err)
	}

	// run projection
	p := eventsourcing.NewProjections(register, json.Unmarshal)
	proj := p.NewRunner(es.GlobalEvents(0, 1), func(event eventsourcing.Event) error {
		return nil
	})

	err, _ = proj.RunOnce()
	if !errors.Is(err, eventsourcing.ErrEventNotRegistered) {
		t.Fatalf("expected ErrEventNotRegistered got %q", err.Error())
	}
}

func TestRace(t *testing.T) {
	// setup
	es := memory.Create()
	register := eventsourcing.NewRegister()
	register.Register(&Person{})

	err := createPersonEvent(es, "kalle", 50)
	if err != nil {
		t.Fatal(err)
	}

	// callback that handles the events
	callbackF := func(event eventsourcing.Event) error {
		time.Sleep(time.Millisecond * 10)
		return nil
	}

	applicationErr := errors.New("an error")

	// run projection
	p := eventsourcing.NewProjections(register, json.Unmarshal)
	p.NewRunner(es.GlobalEvents(0, 1), callbackF)
	p.NewRunner(es.GlobalEvents(0, 1), func(e eventsourcing.Event) error {
		// fail on event 40
		if e.GlobalVersion() == 40 {
			return applicationErr
		}
		return nil
	})

	err = p.Race(true)
	if !errors.Is(err, applicationErr) {
		t.Fatal(err)
	}
}
