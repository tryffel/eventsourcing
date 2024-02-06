package eventsourcing

import (
	"context"
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

}

// Close closes all projections
func (p *Projections) Close() {
	for _, projection := range p.projections {
		projection.Close()
	}
}

// Run runs the projection forever until the context is cancelled
// When there is no more events to concume it sleeps the pace and run again.
func (p *Projection) Run(ctx context.Context, pace time.Duration) error {
	timer := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
			err, work := p.RunOnce()
			if err != nil {
				return err
			}
			// work to do run again ASAP
			if work {
				timer.Reset(0)
				continue
			}
		}
		// no work
		timer.Reset(pace)
	}
}

// RunOnce runs the fetch method one time and returns
func (p *Projection) RunOnce() (error, bool) {
	iterator, err := p.getF()
	if err != nil {
		return err, false
	}

	defer iterator.Close()

	var work bool
	for iterator.Next() {
		work = true
		event, err := iterator.Value()
		if err != nil {
			return err, false
		}

		// TODO: is only registered events of interest?
		f, found := p.projections.register.EventRegistered(event)
		if !found {
			continue
		}

		data := f()
		err = p.projections.deserializer(event.Data, &data)
		if err != nil {
			return err, false
		}

		metadata := make(map[string]interface{})
		if event.Metadata != nil {
			err = p.projections.deserializer(event.Metadata, &metadata)
			if err != nil {
				return err, false
			}
		}

		e := NewEvent(event, data, metadata)
		p.callbackF(e)
	}
	return nil, work
}

func (p *Projection) Close() {
	// TODO stop the projection
}
