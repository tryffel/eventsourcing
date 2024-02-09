package eventsourcing

import (
	"context"
	"errors"
	"fmt"
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
	Name        string
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
		Pace:        time.Second * 10,                  // Default pace 10 seconds
		Strict:      true,                              // Default strict is active
		Name:        fmt.Sprintf("%d", len(p.runners)), // Deafult to the index in the runner slice
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
	for _, runner := range p.runners {
		go func(runner Runner) {
			err := runner.Run(ctx)
			if !errors.Is(err, context.Canceled) {
				errChan <- err
			}
			p.wg.Done()
		}(runner)
	}
	return errChan
}

// Close terminate all running projections
func (p *Projections) Close() {
	p.cancelF()

	// return when all projections has terminated
	p.wg.Wait()
}

type RaceResult struct {
	Error      error
	RunnerName string
}

// Race runs the runners to the end of the there events streams.
// Can be used on a stale event stream with now more events comming in.
func (p *Projections) Race(cancelOnError bool) ([]RaceResult, error) {
	result := make([]RaceResult, len(p.runners))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(len(p.runners))

	var lock sync.Mutex
	var e error

	for i, runner := range p.runners {
		go func(runner Runner, index int) {
			defer wg.Done()
			err := runner.RunToEnd(ctx)
			if err != nil {
				if !errors.Is(err, context.Canceled) && cancelOnError {
					cancel()

					// set the causing error
					lock.Lock()
					e = err
					lock.Unlock()

				}
				lock.Lock()
				result[index] = RaceResult{Error: err, RunnerName: runner.Name}
				lock.Unlock()
			}
		}(runner, i)
	}
	wg.Wait()
	return result, e
}

// Run runs the projection forever until the context is cancelled
// When there is no more events to concume it sleeps the pace and run again.
func (r *Runner) Run(ctx context.Context) error {
	timer := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
			err := r.RunToEnd(ctx)
			if err != nil {
				return err
			}
		}
		// rest
		timer.Reset(r.Pace)
	}
}

// RunToEnd runs until it reach the end of the event stream
func (r *Runner) RunToEnd(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err, ran := r.RunOnce()
			if err != nil {
				return err
			}
			// hit the end of the event stream
			if !ran {
				return nil
			}
		}
	}
}

// RunOnce runs the fetch method one time and returns
func (r *Runner) RunOnce() (error, bool) {
	iterator, err := r.fetchF()
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
		f, found := r.projections.register.EventRegistered(event)
		if !found {
			if r.Strict {
				return ErrEventNotRegistered, false
			}
			continue
		}

		data := f()
		err = r.projections.deserializer(event.Data, &data)
		if err != nil {
			return err, false
		}

		metadata := make(map[string]interface{})
		if event.Metadata != nil {
			err = r.projections.deserializer(event.Metadata, &metadata)
			if err != nil {
				return err, false
			}
		}

		e := NewEvent(event, data, metadata)
		err = r.callbackF(e)
		if err != nil {
			return err, false
		}
	}
	return nil, ran
}
