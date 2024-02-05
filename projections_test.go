package eventsourcing_test

import (
	"encoding/json"
	"testing"

	"github.com/hallgren/eventsourcing"
	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/eventstore/memory"
	"github.com/hallgren/eventsourcing/internal"
)

func Test(t *testing.T) {
	name := ""
	es := memory.Create()
	register := internal.NewRegister()
	register.Register(&Person{})

	person, err := CreatePerson("kalle")
	if err != nil {
		t.Fatal(err)
	}

	events := make([]core.Event, 0)
	for _, e := range person.Events() {
		data, err := json.Marshal(e.Data())
		if err != nil {
			t.Fatal(err)
		}

		events = append(events, core.Event{
			AggregateID:   e.AggregateID(),
			Reason:        e.Reason(),
			AggregateType: e.AggregateType(),
			Version:       core.Version(e.Version()),
			GlobalVersion: core.Version(e.GlobalVersion()),
			Timestamp:     e.Timestamp(),
			Data:          data,
			Metadata:      []byte{},
		})
	}

	es.Save(events)

	p := eventsourcing.NewProjections(register, json.Unmarshal)
	proj := p.Add(es.All(0), func(event eventsourcing.Event) {
		switch e := event.Data().(type) {
		case *Born:
			name = e.Name
		}
	})
	proj.Run()

	if name != person.Name {
		t.Fatalf("expected %q was %q", person.Name, name)
	}
}
