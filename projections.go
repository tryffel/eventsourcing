package eventsourcing

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/internal"
)

type Projection struct {
	getF        func() (core.Iterator, error)
	callbackF   func(e Event)
	projections *Projections
	pace        time.Duration
}

type Projections struct {
	register     *internal.Register // used to map the event types
	deserializer DeserializeFunc
	projections  []Projection
	cancelF      context.CancelFunc
	wg           sync.WaitGroup
}

func NewProjections(register *internal.Register, deserializer DeserializeFunc) *Projections {
	return &Projections{
		register:     register,
		deserializer: deserializer,
		projections:  make([]Projection, 0),
		cancelF:      func() {},
	}
}

func (p *Projections) Add(getF func() (core.Iterator, error), callbackF func(e Event), pace time.Duration) *Projection {
	projection := Projection{
		getF:        getF,
		callbackF:   callbackF,
		projections: p,
		pace:        pace,
	}
	p.projections = append(p.projections, projection)
	return &projection
}

// Start starts all projections and return a channel to notify if a errors is returned from a projection
func (p *Projections) Start() chan error {
	errChan := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelF = cancel

	p.wg.Add(len(p.projections))
	for _, projection := range p.projections {
		go func(proj Projection) {
			err := proj.Run(ctx, proj.pace)
			if !errors.Is(err, context.Canceled) {
				errChan <- err
			}
			p.wg.Done()
		}(projection)
	}
	return errChan
}

// Close terminate all running projections
func (p *Projections) Close() {
	p.cancelF()

	// return when all projections has terminated
	p.wg.Wait()
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
