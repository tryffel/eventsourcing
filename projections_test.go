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
	p := eventsourcing.ProjectionHandler{Register: register, Deserializer: json.Unmarshal}
	proj := p.Projection(es.All(0, 1), func(event eventsourcing.Event) error {
		switch e := event.Data().(type) {
		case *Born:
			projectedName = e.Name
		}
		return nil
	})

	// should set projectedName to kalle
	work, err := proj.RunOnce()
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
	work, err = proj.RunOnce()
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
	p := eventsourcing.ProjectionHandler{Register: register, Deserializer: json.Unmarshal}
	proj := p.Projection(es.All(0, 1), func(event eventsourcing.Event) error {
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

func TestCloseEmptyGroup(t *testing.T) {
	p := eventsourcing.ProjectionHandler{Deserializer: json.Unmarshal}
	g := p.Group()
	g.Close()
}

func TestStartMultipleProjections(t *testing.T) {
	// setup
	es := memory.Create()
	register := eventsourcing.NewRegister()

	// callback that handles the events
	callbackF := func(event eventsourcing.Event) error {
		return nil
	}

	p := eventsourcing.ProjectionHandler{Register: register, Deserializer: json.Unmarshal}
	r1 := p.Projection(es.All(0, 1), callbackF)
	r2 := p.Projection(es.All(0, 1), callbackF)
	r3 := p.Projection(es.All(0, 1), callbackF)

	g := p.Group(r1, r2, r3)
	g.Start()
	g.Close()
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

	p := eventsourcing.ProjectionHandler{Register: register, Deserializer: json.Unmarshal}
	r := p.Projection(es.All(0, 1), callbackF)

	g := p.Group(r)

	errChan := g.Start()
	defer g.Close()

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

	p := eventsourcing.ProjectionHandler{Register: register, Deserializer: json.Unmarshal}
	proj := p.Projection(es.All(0, 1), func(event eventsourcing.Event) error {
		return nil
	})

	_, err = proj.RunOnce()
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
		time.Sleep(time.Millisecond * 2)
		return nil
	}

	applicationErr := errors.New("an error")

	p := eventsourcing.ProjectionHandler{Register: register, Deserializer: json.Unmarshal}
	r1 := p.Projection(es.All(0, 1), callbackF)
	r2 := p.Projection(es.All(0, 1), func(e eventsourcing.Event) error {
		time.Sleep(time.Millisecond)
		if e.GlobalVersion() == 30 {
			return applicationErr
		}
		return nil
	})

	result, err := p.Race(true, r1, r2)

	// causing err should be applicationErr
	if !errors.Is(err, applicationErr) {
		t.Fatalf("expected causing error to be applicationErr got %v", err)
	}

	// projection 0 should have a context.Canceled error
	if !errors.Is(result[0].Error, context.Canceled) {
		t.Fatalf("expected projection %q to have err 'context.Canceled' got %v", result[0].ProjectionName, result[0].Error)
	}

	// projection 1 should have a applicationErr error
	if !errors.Is(result[1].Error, applicationErr) {
		t.Fatalf("expected projection %q to have err 'applicationErr' got %v", result[1].ProjectionName, result[1].Error)
	}

	// projection 1 should have halted on event with GlobalVersion 30
	if result[1].Event.GlobalVersion() != 30 {
		t.Fatalf("expected projection 1 Event.GlobalVersion() to be 30 but was %d", result[1].Event.GlobalVersion())
	}
}

func TestKeepStartPosition(t *testing.T) {
	// setup
	es := memory.Create()
	register := eventsourcing.NewRegister()
	register.Register(&Person{})

	err := createPersonEvent(es, "kalle", 5)
	if err != nil {
		t.Fatal(err)
	}

	start := core.Version(0)
	counter := 0

	// callback that handles the events
	callbackF := func(event eventsourcing.Event) error {
		switch event.Data().(type) {
		case *AgedOneYear:
			counter++
		}
		start = core.Version(event.GlobalVersion() + 1)
		return nil
	}

	p := eventsourcing.ProjectionHandler{Register: register, Deserializer: json.Unmarshal}
	r := p.Projection(es.All(0, 1), callbackF)

	_, err = p.Race(true, r)
	if err != nil {
		t.Fatal(err)
	}

	err = createPersonEvent(es, "anka", 5)
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Race(true, r)
	if err != nil {
		t.Fatal(err)
	}

	// Born 2 + AgedOnYear 5 + 5 = 12 + Next Event 1 = 13
	if start != 13 {
		t.Fatalf("expected start to be 13 was %d", start)
	}

	if counter != 10 {
		t.Fatalf("expected counter to be 10 was %d", counter)
	}
}

/*
func TestCloseRace(t *testing.T) {
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
		time.Sleep(time.Millisecond * 50)
		return nil
	}

	// run projection
	p := eventsourcing.ProjectionHandler{Register: register, Deserializer: json.Unmarshal}
	r := p.Projection(es.GlobalEvents(0, 1), callbackF)

	g := p.RunningGroup()
	g.Add(r)

	var errRace error
	var result []eventsourcing.RaceResult

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		result, errRace = g.Race(true)
		wg.Done()
	}()

	// add sleep to make sure the race setup is done
	time.Sleep(time.Millisecond * 5)
	g.Close()

	// wait for the race to return
	wg.Wait()

	if !errors.Is(errRace, context.Canceled) {
		t.Fatalf("expected a context canceled error on the race got %v", errRace)
	}

	if !errors.Is(result[0].Error, context.Canceled) {
		t.Fatalf("expected a context canceled error on the projection result got %v", result[0].Error)
	}
}
*/
