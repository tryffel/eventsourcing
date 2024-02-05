package eventsourcing

import (
	"context"

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

}

// Close closes all projections
func (p *Projections) Close() {
	for _, projection := range p.projections {
		projection.Close()
	}
}

func (p *Projection) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := p.RunOnce()
			if err != nil {
				return err
			}
		}
	}
}

func (p *Projection) RunOnce() error {
	iterator, err := p.getF()
	if err != nil {
		return err
	}

	defer iterator.Close()

	for iterator.Next() {
		event, err := iterator.Value()
		if err != nil {
			return err
		}

		// TODO: is only registered events of interest?
		f, found := p.projections.register.EventRegistered(event)
		if !found {
			continue
		}

		data := f()
		err = p.projections.deserializer(event.Data, &data)
		if err != nil {
			return err
		}

		metadata := make(map[string]interface{})
		if event.Metadata != nil {
			err = p.projections.deserializer(event.Metadata, &metadata)
			if err != nil {
				return err
			}
		}

		e := NewEvent(event, data, metadata)
		p.callbackF(e)
	}
	return nil
}

func (p *Projection) Close() {
	// TODO stop the projection
}
