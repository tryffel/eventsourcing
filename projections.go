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

type ProjectionHandler struct {
	Register     *Register // used to map the event types
	Deserializer DeserializeFunc
	count        int
}

type Projection struct {
	fetchF    fetchFunc
	callbackF callbackFunc
	handler   *ProjectionHandler
	event     Event
	Pace      time.Duration
	Strict    bool
	Name      string
}

// RunningGroup runs runners concurrently
type RunningGroup struct {
	handler     *ProjectionHandler
	projections []*Projection
	cancelF     context.CancelFunc
	wg          sync.WaitGroup
	lock        sync.Mutex // prevent parallell runs
}

// Projection creates a runner that will run down an event stream
func (p *ProjectionHandler) Projection(fetchF fetchFunc, callbackF callbackFunc) *Projection {
	projection := Projection{
		fetchF:    fetchF,
		callbackF: callbackF,
		handler:   p,
		Pace:      time.Second * 10,           // Default pace 10 seconds
		Strict:    true,                       // Default strict is active
		Name:      fmt.Sprintf("%d", p.count), // Default the name to creation index
	}
	p.count++
	return &projection
}

// Run runs the projection forever until the context is cancelled
// When there is no more events to concume it sleeps the pace and run again.
func (r *Projection) Run(ctx context.Context) error {
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
func (r *Projection) RunToEnd(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			ran, err := r.RunOnce()
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
func (r *Projection) RunOnce() (bool, error) {
	iterator, err := r.fetchF()
	if err != nil {
		return false, err
	}

	defer iterator.Close()

	// ran indicate if there were events to fetch
	var ran bool
	for iterator.Next() {
		ran = true
		event, err := iterator.Value()
		if err != nil {
			return false, err
		}

		// TODO: is only registered events of interest?
		f, found := r.handler.Register.EventRegistered(event)
		if !found {
			if r.Strict {
				return false, fmt.Errorf("event not registered aggregate type: %s, reason: %s, global version: %d, %w", event.AggregateType, event.Reason, event.GlobalVersion, ErrEventNotRegistered)
			}
			continue
		}

		data := f()
		err = r.handler.Deserializer(event.Data, &data)
		if err != nil {
			return false, err
		}

		metadata := make(map[string]interface{})
		if event.Metadata != nil {
			err = r.handler.Deserializer(event.Metadata, &metadata)
			if err != nil {
				return false, err
			}
		}

		e := NewEvent(event, data, metadata)
		// keep a reference to the event currently processing

		r.event = e
		err = r.callbackF(e)
		if err != nil {
			return false, err
		}
	}
	return ran, nil
}

// RunningGroup runs a group of runners concurrently
func (p *ProjectionHandler) RunningGroup() *RunningGroup {
	return &RunningGroup{
		handler:     p,
		projections: make([]*Projection, 0),
		cancelF:     func() {},
	}
}

// Add adds runners to the running group
func (g *RunningGroup) Add(runner ...*Projection) {
	g.projections = append(g.projections, runner...)
}

// Start starts all runners in the running group and return a channel to notify if a errors is returned from a runner
func (g *RunningGroup) Start() chan error {
	g.lock.Lock()
	errChan := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	g.cancelF = cancel

	g.wg.Add(len(g.projections))
	for _, runner := range g.projections {
		go func(runner *Projection) {
			defer g.wg.Done()
			err := runner.Run(ctx)
			if !errors.Is(err, context.Canceled) {
				errChan <- err
			}
		}(runner)
	}
	return errChan
}

// Close terminate all runners in the running group
func (g *RunningGroup) Close() {
	g.cancelF()

	// return when all runners has terminated
	g.wg.Wait()

	// prevent panic if closing a none started running group
	g.lock.TryLock()
	g.lock.Unlock()
}

type RaceResult struct {
	Error      error
	RunnerName string
	Event      Event
}

// Race runs the runners to the end of the there events streams.
// Can be used on a stale event stream with now more events comming in.
func (g *RunningGroup) Race(cancelOnError bool) ([]RaceResult, error) {
	g.lock.Lock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g.cancelF = cancel

	g.wg.Add(len(g.projections))

	var lock sync.Mutex
	var causingErr error

	result := make([]RaceResult, len(g.projections))
	for i, runner := range g.projections {
		go func(runner *Projection, index int) {
			defer g.wg.Done()
			err := runner.RunToEnd(ctx)
			if err != nil {
				if !errors.Is(err, context.Canceled) && cancelOnError {
					cancel()

					lock.Lock()
					causingErr = err
					lock.Unlock()
				}
			}
			lock.Lock()
			result[index] = RaceResult{Error: err, RunnerName: runner.Name, Event: runner.event}
			lock.Unlock()
		}(runner, i)
	}
	g.wg.Wait()
	if causingErr != nil {
		return result, causingErr
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, nil
}
