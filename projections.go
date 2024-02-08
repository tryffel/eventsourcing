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

type Runner struct {
	fetchF      fetchFunc
	callbackF   callbackFunc
	projections *Projections
	Pace        time.Duration
	Strict      bool
}

type Projections struct {
	register     *Register // used to map the event types
	deserializer DeserializeFunc
	runners      []Runner
	cancelF      context.CancelFunc
	wg           sync.WaitGroup
}

func NewProjections(register *Register, deserializer DeserializeFunc) *Projections {
	return &Projections{
		register:     register,
		deserializer: deserializer,
		runners:      make([]Runner, 0),
		cancelF:      func() {},
	}
}

func (p *Projections) NewRunner(fetchF fetchFunc, callbackF callbackFunc) *Runner {
	projection := Runner{
		fetchF:      fetchF,
		callbackF:   callbackF,
		projections: p,
		Pace:        time.Second * 10, // Default pace 10 seconds
		Strict:      true,             // Default strict is active
	}
	p.runners = append(p.runners, projection)
	return &projection
}

// Start starts all projections and return a channel to notify if a errors is returned from a projection
func (p *Projections) Start() chan error {
	errChan := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelF = cancel

	p.wg.Add(len(p.runners))
	for _, projection := range p.runners {
		go func(proj Runner) {
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
func (p *Runner) Run(ctx context.Context) error {
	timer := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
			err, ran := p.RunOnce()
			if err != nil {
				return err
			}
			// it ran last round, run again ASAP.
			if ran {
				timer.Reset(0)
				continue
			}
		}
		// no more running for a while
		timer.Reset(p.Pace)
	}
}

// RunOnce runs the fetch method one time and returns
func (p *Runner) RunOnce() (error, bool) {
	iterator, err := p.fetchF()
	if err != nil {
		return err, false
	}

	defer iterator.Close()

	// ran indicate if there were events to fetch
	var ran bool
	for iterator.Next() {
		ran = true
		event, err := iterator.Value()
		if err != nil {
			return err, false
		}

		// TODO: is only registered events of interest?
		f, found := p.projections.register.EventRegistered(event)
		if !found {
			if p.Strict {
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
	return nil, ran
}
