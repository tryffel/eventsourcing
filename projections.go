package eventsourcing

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hallgren/eventsourcing/core"
)

type fetchFunc func() (core.Iterator, error)
type callbackFunc func(e Event) error

type Projection struct {
	fetchF      fetchFunc
	callbackF   callbackFunc
	projections *Projections
	pace        time.Duration
	strict      bool
}

type Projections struct {
	register     *Register // used to map the event types
	deserializer DeserializeFunc
	projections  []Projection
	cancelF      context.CancelFunc
	wg           sync.WaitGroup
}

func NewProjections(register *Register, deserializer DeserializeFunc) *Projections {
	return &Projections{
		register:     register,
		deserializer: deserializer,
		projections:  make([]Projection, 0),
		cancelF:      func() {},
	}
}

func (p *Projections) Add(fetchF fetchFunc, callbackF callbackFunc, pace time.Duration, strict bool) *Projection {
	projection := Projection{
		fetchF:      fetchF,
		callbackF:   callbackF,
		projections: p,
		pace:        pace,
		strict:      strict,
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
			err := proj.Run(ctx)
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
func (p *Projection) Run(ctx context.Context) error {
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
		timer.Reset(p.pace)
	}
}

// RunOnce runs the fetch method one time and returns
func (p *Projection) RunOnce() (error, bool) {
	iterator, err := p.fetchF()
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
			if p.strict {
				return ErrEventNotRegistered, false
			}
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
		err = p.callbackF(e)
		if err != nil {
			return err, false
		}
	}
	return nil, work
}
