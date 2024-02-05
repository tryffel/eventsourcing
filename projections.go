package eventsourcing

import (
	"time"

	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/internal"
)

type Projection struct {
	getF        func() (core.Iterator, error)
	callbackF   func(e Event)
	projections *Projections
}

type Projections struct {
	register     *internal.Register // used to map the event types
	deserializer DeserializeFunc
	projections  []Projection
}

func NewProjections(register *internal.Register, deserializer DeserializeFunc) *Projections {
	return &Projections{
		register:     register,
		deserializer: deserializer,
		projections:  make([]Projection, 0),
	}
}

func (p *Projections) Add(getF func() (core.Iterator, error), callbackF func(e Event)) *Projection {
	projection := Projection{
		getF:        getF,
		callbackF:   callbackF,
		projections: p,
	}
	p.projections = append(p.projections, projection)
	return &projection
}

// Start starts all projections
func (p *Projections) Start() {
	for _, projection := range p.projections {
		projection.Run()
	}
}

// Close closes all projections
func (p *Projections) Close() {
	for _, projection := range p.projections {
		projection.Close()
	}
}

func (p *Projection) Run() {
	iterator, err := p.getF()
	if err != nil {
		return
	}

	defer iterator.Close()

	work := false
	for iterator.Next() {
		work = true
		event, err := iterator.Value()
		if err != nil {
			return
		}

		// TODO: is only registered events of interest?
		f, found := p.projections.register.EventRegistered(event)
		if !found {
			continue
		}

		data := f()
		err = p.projections.deserializer(event.Data, &data)
		if err != nil {
			return
		}
		/*
			metadata := make(map[string]interface{})
			err = p.projections.deserializer(event.Metadata, &metadata)
			if err != nil {
				return
			}
		*/
		e := NewEvent(event, data, nil)
		p.callbackF(e)
	}
	// sleep if no more events to iterate
	if !work {
		time.Sleep(time.Second * 10)
	}
}

func (p *Projection) Close() {
	// TODO stop the projection
}
